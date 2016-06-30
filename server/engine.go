package server

import (
	"log"
	"time"

	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/reporting"
)

func newSwayEngine(srv *Server) error {
	// Keep a live struct of active campaigns
	// This will be used by "GetAvailableDeals"
	// to avoid constant unmarshalling of campaigns
	srv.Campaigns.Set(srv.db, srv.Cfg)

	cTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range cTicker.C {
			srv.Campaigns.Set(srv.db, srv.Cfg)
		}
	}()

	// Update social media profiles every X hours
	ticker := time.NewTicker(srv.Cfg.EngineUpdate * time.Hour)
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
	// Update all influencer stats/completed deal stats
	if err := updateInfluencers(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with stats updater", err)
		return err
	}
	log.Println("Influencers updated!")

	// Explore the influencer posts to look for completed deals!
	if err := explore(srv); err != nil {
		// Insert a file informant check
		log.Println("Error exploring!", err)
		return err
	}
	log.Println("Posts explored!")

	// Iterate over deltas for completed deals
	// and deplete budgets
	if err := depleteBudget(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with depleting budgets!", err)
		return err
	}
	log.Println("Budgets depleted!")

	return nil
}

func updateInfluencers(s *Server) error {
	activeCampaigns := common.GetAllActiveCampaigns(s.db, s.Cfg)
	influencers := getAllInfluencers(s, false)

	// Traverses all influencers and updates their social media stats
	for _, inf := range influencers {
		// Influencer not updated if they have been updated
		// within the last s.Cfg.InfluencerTTL hours
		if err := inf.UpdateAll(s.Cfg); err != nil {
			return err
		}

		// Inserting a request interval so we don't hit our API
		// limits with platforms!
		time.Sleep(s.Cfg.StatsInterval * time.Second)

		// Update data for all completed deal posts
		if err := inf.UpdateCompletedDeals(s.Cfg, activeCampaigns); err != nil {
			return err
		}

		time.Sleep(s.Cfg.StatsInterval * time.Second)

		if err := saveAllDeals(s, inf); err != nil {
			return err
		}
	}

	return nil
}

func depleteBudget(s *Server) error {
	// now that we have updated stats for completed deals
	// go over completed deals..
	// Iterate over all

	var spentDelta float64

	activeCampaigns := common.GetAllActiveCampaigns(s.db, s.Cfg)
	// Iterate over all active campaigns
	for _, cmp := range activeCampaigns {
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
				payout := spentDelta * (1 - agencyFee)
				inf.PendingPayout += payout

				// Update payment values for this completed deal
				for _, cDeal := range inf.CompletedDeals {
					if cDeal.Id == deal.Id {
						cDeal.Pay(payout, spentDelta-payout, inf.AgencyId)
					}
				}

				// Save the deal in influencers and campaigns
				if err := saveAllDeals(s, inf); err != nil {
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
