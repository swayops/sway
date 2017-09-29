package server

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

const EngineRunTime = 4

func newSwayEngine(srv *Server) error {
	// Keep a live struct of active campaigns
	// This will be used by "GetAvailableDeals"
	// to avoid constant unmarshalling of campaigns

	// getActiveAdvertisers only returns advertisers which are on
	// and have valid subscriptions!
	srv.Campaigns.Set(srv.db, srv.Cfg, getActiveAdvertisers(srv), getActiveAdAgencies(srv), getFeesByAdv(srv))
	cTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range cTicker.C {
			srv.Campaigns.Set(srv.db, srv.Cfg, getActiveAdvertisers(srv), getActiveAdAgencies(srv), getFeesByAdv(srv))
		}
	}()

	if err := fillConversions(srv); err != nil {
		return err
	}
	convTicker := time.NewTicker(40 * time.Minute)
	go func() {
		for range convTicker.C {
			// Every 40 minutes.. go in and fill conversion
			// values
			if err := fillConversions(srv); err != nil {
				srv.Alert("Error runnin conversion fill", err)
			}
		}
	}()

	srv.Audiences.Set(srv.db, srv.Cfg, getFollowersByEmail(srv))
	audTicker := time.NewTicker(40 * time.Minute)
	go func() {
		for range audTicker.C {
			srv.Audiences.Set(srv.db, srv.Cfg, getFollowersByEmail(srv))
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

	// Keep a live struct for all scraps in the platform
	srv.Scraps.Set(srv.db, srv.Cfg, getAllScraps(srv))
	scrapsTicker := time.NewTicker(1 * time.Hour)
	go func() {
		for range scrapsTicker.C {
			srv.Scraps.Set(srv.db, srv.Cfg, getAllScraps(srv))
		}
	}()

	// Run engine every X hours
	runTicker := time.NewTicker(EngineRunTime * time.Hour)
	go func() {
		for range runTicker.C {
			if err := run(srv); err != nil {
				log.Println("Err running engine", err)
			}
		}
	}()

	// Check social media keys every 30 minutes!
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

			if _, err := youtube.New("UCK8sQmJBp8GCxrOtXWBpyEA", srv.Cfg); err != nil {
				srv.Alert("Error running YouTube init!", err)
			}

			// if testGeo := geo.GetGeoFromCoords(34.1341, -118.3215, int32(time.Now().Unix())); testGeo == nil || testGeo.State != "CA" {
			// 	srv.Alert("Error hitting Google geo!", nil)
			// }

			// Ping click URLs
			if err := misc.Request("GET", "https://swayops.com/cl/fakeID", "", nil); err != nil {
				srv.Alert("Error hitting First Click URL!", err)
			}

			if err := misc.Request("GET", "https://swayops.com/c/fakeID", "", nil); err != nil {
				srv.Alert("Error hitting Second Click URL!", err)
			}

			if err := misc.Ping("https://dash.swayops.com/api/v1/images/sway_logo.png"); err != nil {
				srv.Alert("Error hitting Sway logo!", err)
			}
		}
	}()

	// Add keywords to scraps/influencers every 4 hours
	attrTicker := time.NewTicker(96 * time.Hour)
	go func() {
		for range attrTicker.C {
			if _, err := attributer(srv, false); err != nil {
				srv.Alert("Err running scrap attributer", err)
			}
		}
	}()

	// Save pictures every 4 hours
	imageTicker := time.NewTicker(4 * time.Hour)
	go func() {
		imageSaver(srv)
		for range imageTicker.C {
			imageSaver(srv)
		}
	}()

	billingTicker := time.NewTicker(24 * time.Hour)
	go func() {
		if err := srv.billing(); err != nil {
			srv.Alert("Err running billing notifier", err)
		}

		for range billingTicker.C {
			if err := srv.billing(); err != nil {
				srv.Alert("Err running billing notifier", err)
			}
		}
	}()

	return nil
}

type Depleted struct {
	Influencer string  `json:"inf,omitempty"`
	Campaign   string  `json:"campaign,omitempty"`
	PostURL    string  `json:"postURL,omitempty"`
	Spent      float64 `json:"spent,omitempty"`
}

func run(srv *Server) error {
	log.Println("Beginning engine run!")

	var (
		err                                                 error
		updatedInf, foundDeals, dealsEmailed, scrapsEmailed int32
		depletions                                          []*Depleted
	)

	// NOTE: This is the only function that can and should edit
	// budget and reporting DBs
	start := time.Now()

	// // Lets just check for any completed signatures right off the bat!
	// if sigsFound, err = auditTaxes(srv); err != nil {
	// 	srv.Alert("Error auditing taxes!", err)
	// 	return err
	// }

	// log.Println("Taxes audited. Found:", sigsFound)

	// Update all influencer stats/completed deal stats
	// If anything fails to update.. just stop here
	// This ensures that Deltas aren't accounted for twice
	// in the case someting errors out and continues!
	if updatedInf, err = updateInfluencers(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Stats update failed!", err)
		return err
	}

	// Lets confirm that there are budget keys
	// for the new month before we kick this off.
	// This is for the case that it's the first
	// of the month and billing hasnt run yet
	if !shouldRun(srv) {
		return nil
	}

	log.Println("Completed influencer update. Updated:", updatedInf)

	// Explore the influencer posts to look for completed deals!
	if foundDeals, err = explore(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Exploring influencer posts failed!", err)
		return err
	}

	log.Println("Completed deal exploration. Found:", foundDeals)

	// Iterate over deltas for completed deals
	// and deplete budgets
	if depletions, err = depleteBudget(srv); err != nil {
		// Insert a file informant check
		srv.Alert("Error depleting budget!", err)
		return err
	}

	log.Println("Budgets depleted. Depleted:", len(depletions))

	if dealsEmailed, err = emailDeals(srv); err != nil {
		srv.Alert("Error emailing deals!", err)
		return err
	}

	log.Println("Deals emailed. Sent:", dealsEmailed)

	if scrapsEmailed, err = emailScraps(srv); err != nil {
		srv.Alert("Error emailing scraps!", err)
		return err
	}

	log.Println("Scraps emailed. Sent:", scrapsEmailed)

	srv.Digest(updatedInf, foundDeals, depletions, dealsEmailed, scrapsEmailed, start)

	srv.Stats.Update(updatedInf, time.Now().Unix())

	return nil
}

var ErrStore = errors.New("Empty budget store!")

func shouldRun(s *Server) bool {
	// Iterate over all active campaigns
	for _, cmp := range s.Campaigns.GetStore() {
		if cmp.IsValid() {
			return true
		}
	}
	// s.Alert("No active campaigns with budget!", ErrStore)
	return false
}

func updateInfluencers(s *Server) (int32, error) {
	activeCampaigns := s.Campaigns.GetStore()

	var (
		oldUpdate int32
		err       error
		updated   int32
	)
	for _, infId := range s.auth.Influencers.GetAllIDs() {
		var private bool

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
			if private {
				log.Println("Failed to update private influencer "+infId, err)
			} else {
				log.Println("Failed to update influencer "+infId, err)
			}

			if private {
				// We noticed that this influencer now has a private profile..
				// lets let them know!
				if err = inf.PrivateEmail(s.Cfg); err != nil {
					s.Alert("Private email failed", err)
					continue
				}
				inf.PrivateNotify = int32(time.Now().Unix())
			}
		}

		// Inserting a request interval so we don't hit our API
		// limits with platforms!
		if inf.LastSocialUpdate != oldUpdate {
			// Only sleep if the influencer was actually updated!
			time.Sleep(1 * time.Second)
			updated += 1
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

	}

	return updated, nil
}

func depleteBudget(s *Server) ([]*Depleted, error) {
	// now that we have updated stats for completed deals
	// go over completed deals..
	// Iterate over all

	var (
		spentDelta float64
		m          *budget.Metrics
	)

	depletions := []*Depleted{}

	// Iterate over all active campaigns
	for _, cmp := range s.Campaigns.GetStore() {
		// Get this month's store for this campaign
		store, err := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || store == nil || store.IsClosed(&cmp) {
			if !s.Cfg.Sandbox {
				s.Alert("Could not find store for "+cmp.Id, errors.New("Could not find store"))
			}
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

			dspMarkup, exchangeMarkup, agencyPayout, infPayout := budget.GetMargins(spentDelta, dspFee, exchangeFee, agencyFee)

			inf.PendingPayout += infPayout

			// Update payment values for this completed deal
			// THIS IS WHAT WE'LL USE FOR BILLING!
			for _, cDeal := range inf.CompletedDeals {
				if cDeal.Id == deal.Id {
					cDeal.Incr(m.Likes, m.Dislikes, m.Comments, m.Shares, m.Views)
					cDeal.Pay(infPayout, agencyPayout, dspMarkup, exchangeMarkup, inf.AgencyId)
					cDeal.ApproveAllClicks()

					// Log the incrs!
					if infPayout+agencyPayout+dspMarkup+exchangeMarkup > 0 {
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

				// Used for digest email!
				// NOTE: Only email if spent is more than 50 cents
				if spentDelta > 0.10 {
					depletions = append(depletions, &Depleted{
						Influencer: fmt.Sprintf("%s (%s)", deal.InfluencerName, deal.InfluencerId),
						Campaign:   fmt.Sprintf("%s (%s)", deal.CampaignName, deal.CampaignId),
						PostURL:    deal.PostUrl,
						Spent:      misc.TruncateFloat(spentDelta, 2)})
				}
			}

			updatedStore = true
		}

		if updatedStore {
			// Save the store since it's been updated
			if err := budget.SaveStore(s.db, s.Cfg, store, &cmp); err != nil {
				s.Alert("Failed to save budget store", err)
			}
		}

	}

	return depletions, nil
}

// func auditTaxes(srv *Server) (int32, error) {
// 	var (
// 		sigsFound int32
// 	)

// 	for _, infId := range srv.auth.Influencers.GetAllIDs() {
// 		// Do another get incase the influencer has been updated
// 		// and since this iteration could take a while
// 		inf, ok := srv.auth.Influencers.Get(infId)
// 		if !ok {
// 			continue
// 		}

// 		if inf.SignatureId != "" && !inf.HasSigned {
// 			val, err := hellosign.HasSigned(inf.Id, inf.SignatureId)
// 			if err != nil {
// 				srv.Alert("Error from HelloSign for "+inf.SignatureId, err)
// 				continue
// 			}
// 			if inf.HasSigned != val {
// 				if err := srv.db.Update(func(tx *bolt.Tx) error {
// 					inf.HasSigned = val
// 					if val {
// 						sigsFound += 1
// 					}
// 					// Save the influencer since we just updated it's social media data
// 					if err := saveInfluencer(srv, tx, inf); err != nil {
// 						log.Println("Errored saving influencer", err)
// 						return err
// 					}
// 					return nil
// 				}); err != nil {
// 					log.Println("Error when saving influencer", err)
// 					return sigsFound, err
// 				}
// 			}
// 		}
// 	}

// 	return sigsFound, nil
// }

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
		if emailed, err = inf.Email(s.Campaigns, s.Audiences, s.db, s.getTalentAgencyFee(inf.AgencyId), s.Cfg); err != nil {
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

		if infEmails > 40 {
			// No more than 40 emails per run
			break
		}
	}

	return infEmails, nil
}
