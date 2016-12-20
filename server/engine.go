package server

import (
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/hellosign"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

func newSwayEngine(srv *Server) error {
	// Keep a live struct of active campaigns
	// This will be used by "GetAvailableDeals"
	// to avoid constant unmarshalling of campaigns
	srv.Campaigns.Set(srv.db, srv.Cfg, getActiveAdvertisers(srv), getActiveAdAgencies(srv))
	cTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range cTicker.C {
			srv.Campaigns.Set(srv.db, srv.Cfg, getActiveAdvertisers(srv), getActiveAdAgencies(srv))
		}
	}()

	// Keep a live struct for all influencers in the platform
	srv.auth.Influencers.Set(getAllInfluencers(srv))
	infTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range infTicker.C {
			srv.auth.Influencers.Set(getAllInfluencers(srv))
		}
	}()

	// Run engine every 3 hours
	runTicker := time.NewTicker(3 * time.Hour)
	go func() {
		for range runTicker.C {
			if err := run(srv); err != nil {
				log.Println("Err running engine", err)
			}
		}
	}()

	// Check social media keys every hour!
	addr := &lob.AddressLoad{"917 HARTFORD WAY", "", "BEVERLY HILLS", "CA", "US", "90210"}
	alertTicker := time.NewTicker(30 * time.Minute)
	go func() {
		for range alertTicker.C {
			if _, err := facebook.New("facebook", srv.Cfg); err != nil {
				srv.Alert("Error running Facebook init!", err)
			}

			if !instagram.Status(srv.Cfg) {
				srv.Alert("Error running Instagram init!", instagram.ErrUnknown)
			}

			if !twitter.Status(srv.Cfg) {
				srv.Alert("Error running Twitter init!", twitter.ErrEligible)
			}

			if _, err := youtube.New("google", srv.Cfg); err != nil {
				srv.Alert("Error running YouTube init!", err)
			}

			if _, err := lob.VerifyAddress(addr, false); err != nil {
				srv.Alert("Error hitting LOB!", err)
			}

			if testGeo := geo.GetGeoFromCoords(34.1341, -118.3215, int32(time.Now().Unix())); testGeo == nil || testGeo.State != "CA" {
				srv.Alert("Error hitting Google geo!", nil)
			}
		}
	}()

	// Add keywords to scraps/influencers every 4 hours
	attrTicker := time.NewTicker(4 * time.Hour)
	go func() {
		for range attrTicker.C {
			if _, err := attributer(srv, false); err != nil {
				srv.Alert("Err running scrap attributer", err)
			}
		}
	}()

	return nil
}

func run(srv *Server) error {
	log.Println("Beginning engine run!")

	var (
		err                                                            error
		updatedInf, foundDeals, sigsFound, dealsEmailed, scrapsEmailed int32
		totalDepleted                                                  float64
	)

	// NOTE: This is the only function that can and should edit
	// budget and reporting DBs
	start := time.Now()

	// Lets confirm that there are budget keys
	// for the new month before we kick this off.
	// This is for the case that it's the first
	// of the month and billing hasnt run yet
	if !shouldRun(srv) {
		return nil
	}

	// Update all influencer stats/completed deal stats
	// If anything fails to update.. just stop here
	// This ensures that Deltas aren't accounted for twice
	// in the case someting errors out and continues!
	if updatedInf, err = updateInfluencers(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Stats update failed!", err)
		return err
	}

	// Explore the influencer posts to look for completed deals!
	if foundDeals, err = explore(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Exploring influencer posts failed!", err)
		return err
	}

	// Iterate over deltas for completed deals
	// and deplete budgets
	if totalDepleted, err = depleteBudget(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Error depleting budget!", err)
		return err
	}

	if sigsFound, err = auditTaxes(srv); err != nil {
		srv.Alert("Error auditing taxes!", err)
		return err
	}

	if dealsEmailed, err = emailDeals(srv); err != nil {
		srv.Alert("Error emailing deals!", err)
		return err
	}

	if scrapsEmailed, err = emailScraps(srv); err != nil {
		srv.Alert("Error emailing scraps!", err)
		return err
	}

	srv.Digest(updatedInf, foundDeals, totalDepleted, sigsFound, dealsEmailed, scrapsEmailed, start)

	return nil
}

var ErrStore = errors.New("Empty budget store!")

func shouldRun(s *Server) bool {
	// Iterate over all active campaigns
	for _, cmp := range s.Campaigns.GetStore() {
		// Get this month's store for this campaign
		// If there's even one that's AVAILABLE
		// it means billing HAS run so lets
		// CONTINUE the engine
		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
		if err == nil && store != nil {
			return true
		}
	}
	s.Alert("No active campaigns with budget!", ErrStore)
	return false
}

func updateInfluencers(s *Server) (int32, error) {
	activeCampaigns := s.Campaigns.GetStore()

	var (
		oldUpdate int32
		err       error
		updated   int32
		private   bool
	)
	for _, infId := range s.auth.Influencers.GetAllIDs() {
		// Do another get incase the influencer has been updated
		// and since this iteration could take a while
		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			continue
		}

		oldUpdate = inf.LastSocialUpdate

		// Influencer not updated if they have been updated
		// within the last 12 hours
		if private, err = inf.UpdateAll(s.Cfg); err != nil {
			// If the update errors.. we continue and alert
			// admin about the error. Do not return because
			// we clear out engagement deltas anyway
			// whenever we deplete budgets so don't want to stop
			// the whole engine because of one influencer erroring
			log.Println("Failed to update influencer "+infId, err)

			if private {
				// We noticed that this influencer now has a private profile..
				// lets let them know!
				if err = inf.PrivateEmail(s.Cfg); err != nil {
					s.Alert("Private email failed", err)
					continue
				}

				if err = updatePrivateEmailNotification(s, inf.Id); err != nil {
					log.Println("Saving private email notificationf ailed")
				}
			}
			continue
		}

		// Inserting a request interval so we don't hit our API
		// limits with platforms!
		if inf.LastSocialUpdate != oldUpdate {
			// Only sleep if the influencer was actually updated!
			time.Sleep(1 * time.Second)
		}

		// Update data for all completed deal posts
		if err = inf.UpdateCompletedDeals(s.Cfg, activeCampaigns); err != nil {
			s.Alert("Failed to update complete deals for "+infId, err)
			continue
		}

		// Also saves influencers!
		if err = saveAllCompletedDeals(s, inf); err != nil {
			return updated, err
		}

		if len(inf.CompletedDeals) > 0 {
			// If inf had completed deals..they were most likely updated
			// Lets sleep for a bit just incase!
			time.Sleep(1 * time.Second)
		}

		updated += 1
	}

	return updated, nil
}

func depleteBudget(s *Server) (float64, error) {
	// now that we have updated stats for completed deals
	// go over completed deals..
	// Iterate over all

	var (
		spentDelta    float64
		m             *budget.Metrics
		totalDepleted float64
	)

	// Iterate over all active campaigns
	for _, cmp := range s.Campaigns.GetStore() {
		// Get this month's store for this campaign
		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
		if err != nil {
			log.Println("Could not find store for "+cmp.Id, errors.New("Could not find store"))
			continue
		}
		updatedStore := false

		dspFee, exchangeFee := getAdvertiserFees(s.auth, cmp.AdvertiserId)
		// Look for any completed deals
		for _, deal := range cmp.Deals {
			if deal.Completed == 0 {
				continue
			}

			inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
			if !ok {
				log.Println("Missing influencer!", deal.InfluencerId)
				continue
			}

			agencyFee := s.getTalentAgencyFee(inf.AgencyId)
			store, spentDelta, m = budget.AdjustStore(store, deal)
			// Save the influencer since pending payout has been increased
			if spentDelta > 0 {
				totalDepleted += spentDelta

				// DSP and Exchange fee taken away from the prinicpal
				dspMarkup := spentDelta * dspFee
				exchangeMarkup := spentDelta * exchangeFee

				// Talent agency payout will be taken away from the influencer portion
				influencerPool := spentDelta - (dspMarkup + exchangeMarkup)
				agencyPayout := influencerPool * agencyFee
				infPayout := influencerPool - agencyPayout

				inf.PendingPayout += infPayout

				// Update payment values for this completed deal
				// THIS IS WHAT WE'LL USE FOR BILLING!
				for _, cDeal := range inf.CompletedDeals {
					if cDeal.Id == deal.Id {
						cDeal.Incr(m.Likes, m.Dislikes, m.Comments, m.Shares, m.Views)
						cDeal.Pay(infPayout, agencyPayout, dspMarkup, exchangeMarkup, inf.AgencyId)

						// Log the incrs!
						if err := s.Cfg.Loggers.Log("stats", map[string]interface{}{
							"action":     "deplete",
							"infId":      cDeal.InfluencerId,
							"dealId":     cDeal.Id,
							"campaignId": cDeal.CampaignId,
							"agencyId":   inf.AgencyId,
							"stats":      m,
							"payouts": map[string]float64{
								"inf":      infPayout,
								"agency":   agencyPayout,
								"dsp":      dspMarkup,
								"exchange": exchangeMarkup,
							},
							"store": store,
						}); err != nil {
							log.Println("Failed to log appproved deal!", cDeal.InfluencerId, cDeal.CampaignId)
						}
					}
				}

				// infPayout = what the influencer will be earning (not including agency fee)
				// agencyPayout = what the talent agency will be earning
				// dspMarkup = what the DSP fee is for this transaction
				// exchangeMarkup = what the exchange fee is for this transaction

				// Save the deal in influencers and campaigns
				if err := saveAllCompletedDeals(s, inf); err != nil {
					// Insert file informant notification
					s.Alert("Failed to save completed deals", err)
				}
			}

			updatedStore = true
		}

		if updatedStore {
			// Save the store since it's been updated
			if err := budget.SaveStore(s.budgetDb, s.Cfg, store, cmp.Id); err != nil {
				s.Alert("Failed to save budget store", err)
			}
		}

	}

	return totalDepleted, nil
}

func auditTaxes(srv *Server) (int32, error) {
	var (
		sigsFound int32
	)

	for _, infId := range srv.auth.Influencers.GetAllIDs() {
		// Do another get incase the influencer has been updated
		// and since this iteration could take a while
		inf, ok := srv.auth.Influencers.Get(infId)
		if !ok {
			continue
		}

		if inf.SignatureId != "" && !inf.HasSigned {
			val, err := hellosign.HasSigned(inf.Id, inf.SignatureId)
			if err != nil {
				srv.Alert("Error from HelloSign for "+inf.SignatureId, err)
				continue
			}
			if inf.HasSigned != val {
				if err := srv.db.Update(func(tx *bolt.Tx) error {
					inf.HasSigned = val
					if val {
						sigsFound += 1
					}
					// Save the influencer since we just updated it's social media data
					if err := saveInfluencer(srv, tx, inf); err != nil {
						log.Println("Errored saving influencer", err)
						return err
					}
					return nil
				}); err != nil {
					log.Println("Error when saving influencer", err)
					return sigsFound, err
				}
			}
		}
	}

	return sigsFound, nil
}

func emailDeals(s *Server) (int32, error) {
	// Email Influencers
	var infEmails int32
	for _, inf := range s.auth.Influencers.GetAll() {
		if !inf.DealPing {
			continue
		}

		var (
			emailed bool
			err     error
		)
		if emailed, err = inf.Email(s.Campaigns, s.budgetDb, s.Cfg); err != nil {
			log.Println("Error emailing influencer!", err)
			continue
		}

		// Don't save TS if we didnt email foo!
		if !emailed {
			continue
		}

		infEmails += 1
		// Save the last email timestamp
		if err := updateLastEmail(s, inf.Id); err != nil {
			log.Println("Error when saving influencer", err, inf.Id)
		}
	}

	return infEmails, nil
}
