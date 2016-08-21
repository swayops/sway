package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/hellosign"
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

	// Run engine every 6 hours
	ticker := time.NewTicker(6 * time.Hour)
	go func() {
		for range ticker.C {
			if err := run(srv); err != nil {
				log.Println("Err running engine", err)
			}
		}
	}()
	return nil
}

func run(srv *Server) error {
	// NOTE: This is the only function that can and should edit
	// budget and reporting DBs
	log.Println("Initiating engine run @", time.Now().String())

	// Update all influencer stats/completed deal stats
	if err := updateInfluencers(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with stats updater", err)
		return err
	}

	// Explore the influencer posts to look for completed deals!
	if err := explore(srv); err != nil {
		// Insert a file informant check
		log.Println("Error exploring!", err)
		return err
	}

	// Iterate over deltas for completed deals
	// and deplete budgets
	if err := depleteBudget(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with depleting budgets!", err)
		return err
	}

	if err := auditTaxes(srv); err != nil {
		log.Println("Err with auditing taxes!", err)
		return err
	}

	if err := emailDeals(srv); err != nil {
		log.Println("Err with emailing deals!", err)
		return err
	}

	log.Println("Completed engine run @", time.Now().String(), "\n")
	return nil
}

func updateInfluencers(s *Server) error {
	activeCampaigns := s.Campaigns.GetStore()
	influencers := getAllInfluencers(s)

	var (
		inf       *auth.Influencer
		oldUpdate int32
		err       error
	)
	for _, infId := range influencers {
		// Do another get incase the influencer has been updated
		// and since this iteration could take a while
		inf = s.auth.GetInfluencer(infId)
		if inf == nil {
			continue
		}

		oldUpdate = inf.LastSocialUpdate

		// Influencer not updated if they have been updated
		// within the last 12 hours
		if err = inf.UpdateAll(s.Cfg); err != nil {
			return err
		}

		// Inserting a request interval so we don't hit our API
		// limits with platforms!
		if inf.LastSocialUpdate != oldUpdate {
			// Only sleep if the influencer was actually updated!
			time.Sleep(3 * time.Second)
		}

		// Update data for all completed deal posts
		if err = inf.UpdateCompletedDeals(s.Cfg, activeCampaigns); err != nil {
			return err
		}

		// Also saves influencers!
		if err = saveAllCompletedDeals(s, inf); err != nil {
			return err
		}

		if len(inf.CompletedDeals) > 0 {
			// If inf had completed deals..they were most likely updated
			// Lets sleep for a bit just incase!
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}

func depleteBudget(s *Server) error {
	// now that we have updated stats for completed deals
	// go over completed deals..
	// Iterate over all

	var spentDelta float64

	// Iterate over all active campaigns
	for _, cmp := range s.Campaigns.GetStore() {
		// Get this month's store for this campaign
		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
		if err != nil {
			log.Println("Could not find store!", cmp.Id)
			continue
		}
		updatedStore := false

		// Look for any completed deals
		for _, deal := range cmp.Deals {
			if deal.Completed == 0 {
				continue
			}
			inf := s.auth.GetInfluencer(deal.InfluencerId)
			if inf == nil {
				log.Println("Missing influencer!")
				continue
			}

			// Get stats for this deal for today! We'll need it so that stats
			// can be incremented in AdjustStore!
			stats, statsKey, err := reporting.GetStats(deal, s.reportingDb, s.Cfg, inf.GetPlatformId(deal))
			if err != nil {
				// Insert file informant notification
				log.Println("Unable to retrieve stats!")
				continue
			}

			agencyFee := s.getTalentAgencyFee(inf.AgencyId)
			store, stats, spentDelta = budget.AdjustStore(store, deal, stats)

			// Save the influencer since pending payout has been increased
			if spentDelta > 0 {
				infPayout := spentDelta * (1 - agencyFee)
				agencyPayout := spentDelta - infPayout
				inf.PendingPayout += infPayout

				// Update payment values for this completed deal
				for _, cDeal := range inf.CompletedDeals {
					if cDeal.Id == deal.Id {
						cDeal.Pay(infPayout, agencyPayout, inf.AgencyId)
					}
				}

				stats.InfPayout += infPayout
				stats.AgencyPayout += agencyPayout
				stats.TalentAgency = inf.AgencyId

				// Save the deal in influencers and campaigns
				if err := saveAllCompletedDeals(s, inf); err != nil {
					// Insert file informant notification
					log.Println("Error saving deals!", err)
				}
			}
			if err := reporting.SaveStats(stats, deal, s.reportingDb, s.Cfg, statsKey, inf.GetPlatformId(deal)); err != nil {
				log.Println("Err saving stats!", err)
			}

			updatedStore = true
		}

		if updatedStore {
			// Save the store since it's been updated
			if err := budget.SaveStore(s.budgetDb, s.Cfg, store, cmp.Id); err != nil {
				log.Println("Err saving store!", err)
			}
		}

	}

	return nil
}

func auditTaxes(srv *Server) error {
	var (
		sigsFound int32
		inf       *auth.Influencer
	)

	influencers := getAllInfluencers(srv)
	for _, infId := range influencers {
		// Do another get incase the influencer has been updated
		// and since this iteration could take a while
		inf = srv.auth.GetInfluencer(infId)
		if inf == nil {
			continue
		}

		if inf.SignatureId != "" && !inf.HasSigned {
			val, err := hellosign.HasSigned(inf.Id, inf.SignatureId)
			if err != nil {
				log.Println("Error from HelloSign", err)
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
					return err
				}
			}
		}
	}
	if sigsFound > 0 {
		log.Println(sigsFound, "signatures found!\n")
	}
	return nil
}

func emailDeals(s *Server) error {
	if !s.Cfg.Sandbox {
		log.Println("Initiating email run @", time.Now().String())
	}

	// Email Influencers
	var influencers []*auth.Influencer
	// If an influencer was made in the last day
	// this map will be added to.. and will be used
	// to delete that scrap later in this func
	emailMap := make(map[string]bool)

	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
			if inf := auth.GetInfluencer(u); inf != nil {
				if misc.WithinLast(inf.CreatedAt, 24) {
					emailMap[inf.EmailAddress] = true
				}

				if inf.DealPing {
					influencers = append(influencers, inf)
				}
			}
			return nil
		})
	})

	var infEmails int32
	for _, inf := range influencers {
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
		if err := s.db.Update(func(tx *bolt.Tx) error {
			inf.LastEmail = int32(time.Now().Unix())
			// Save the influencer since we just updated it's social media data
			if err := saveInfluencer(s, tx, inf); err != nil {
				log.Println("Errored saving influencer", err)
				return err
			}
			return nil
		}); err != nil {
			log.Println("Error when saving influencer", err)
		}
	}

	// Email Scraps
	var (
		scraps                 []*influencer.Scrap
		scrapEmails, deletions int32
	)
	if err := s.db.Update(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc influencer.Scrap
			if err := json.Unmarshal(v, &sc); err != nil {
				log.Println("error when unmarshalling scrap", string(v))
				return nil
			}
			signedUp := emailMap[sc.EmailAddress]
			// Delete the user if they've been sent 3 emails now
			// OR they've signed up as an influencer
			if signedUp || len(sc.SentEmails) == 3 {
				deletions += 1
				return misc.DelBucketBytes(tx, s.Cfg.Bucket.Scrap, sc.Id)
			} else {
				scraps = append(scraps, &sc)
			}
			return
		})
		return nil
	}); err != nil {
		return err
	}

	for _, sc := range scraps {
		if err := sc.Email(s.Campaigns, s.db, s.budgetDb, s.Cfg); err != nil {
			log.Println("Error emailing scrap!", err)
			continue
		}
		scrapEmails += 1
	}

	if !s.Cfg.Sandbox {
		log.Println(len(emailMap), "influencers signed up over the last 2 days")
		log.Println(infEmails, "influencers emailed")
		log.Println(scrapEmails, "scraps emailed, and", deletions, "scraps deleted")
		log.Println("Finished email run @", time.Now().String(), "\n")
	}

	return nil
}
