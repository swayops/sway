package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
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

	// Will run the first of every month to
	// refresh our budget dbs, budgets,
	// and send out invoices
	if err := billing(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with billing!", err)
		return err
	}

	return nil
}

func updateInfluencers(s *Server) error {
	activeCampaigns := common.GetAllActiveCampaigns(s.db, s.Cfg)
	influencers := influencer.GetAllInfluencers(s.db, s.Cfg)

	// Traverses all influencers and updates their social media stats
	for _, inf := range influencers {
		if inf.Id != "1" {
			continue
		}
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

			inf, err := influencer.GetInfluencerFromId(deal.InfluencerId, s.db, s.Cfg)
			if err != nil {
				log.Println("Missing influencer id!")
				continue
			}

			// Get stats for this deal for today! We'll need it to increment
			// stat values in AdjustStore!
			stats, statsKey, err := reporting.GetStats(deal, s.reportingDb, s.Cfg, inf.GetPlatformId(deal))
			if err != nil {
				// Insert file informant notification
				log.Println("Unable to retrieve stats!")
				continue
			}

			store, stats = budget.AdjustStore(store, deal, stats)
			// Save the stats store since it's been updated
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

const (
	cmpInvoiceFormat          = "Campaign ID: %s, Email: test@sway.com, Phone: 123456789, Spent: %f, DSPFee: %f, ExchangeFee: %f, Total: %f"
	infInvoiceFormat          = "Influencer Name: %s, Influencer ID: %s, Email: test@sway.com, Payout: %f"
	talentAgencyInvoiceFormat = "Agency Name: %s, Agency ID: %s, Email: test@sway.com, Payout: %f"
)

var ErrEmptyStore = errors.New("Empty store when billing!")

func billing(s *Server) error {
	// If it's the first day and billing
	// has not yet run in the last week...
	if budget.ShouldBill(s.budgetDb, s.Cfg) {
		// Now that it's a new month.. get last month's budget store
		store, err := budget.GetStore(s.budgetDb, s.Cfg, budget.GetLastMonthBudgetKey())
		if err != nil || len(store) == 0 {
			// Insert file informant check
			return ErrEmptyStore
		}

		// Campaign Invoice
		log.Println("Advertiser Invoice")
		for cId, data := range store {
			formatted := fmt.Sprintf(
				cmpInvoiceFormat,
				cId,
				data.Spent,
				data.DspFee,
				data.ExchangeFee,
				data.Spent+data.DspFee+data.ExchangeFee,
			)
			log.Println(formatted)
		}

		// Influencer Invoice
		log.Println("Influencer Invoice")
		for _, data := range store {
			for id, infData := range data.Influencers {
				var (
					inf       influencer.Influencer
					v         []byte
					agencyFee float32
				)
				if err := s.db.View(func(tx *bolt.Tx) error {
					v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id))
					return nil
				}); err != nil {
					log.Println("Invoice error for influencer", err)
					continue
				}

				if err = json.Unmarshal(v, &inf); err != nil {
					log.Println("Invoice error for influencer", err)
					continue
				}

				agencyFee = getTalentAgencyFee(s, inf.AgencyId)
				if agencyFee == 0 {
					log.Println("error retrieving agency fee for", inf.Id)
					continue
				}

				formatted := fmt.Sprintf(
					infInvoiceFormat,
					inf.Name,
					id,
					infData.Payout*(1-agencyFee),
				)
				log.Println(formatted)
			}
		}

		// Talent Agency Invoice
		log.Println("Talent Agency Invoice")
		for _, data := range store {
			for id, infData := range data.Influencers {
				var (
					ag        common.TalentAgency
					inf       influencer.Influencer
					v         []byte
					agencyFee float32
				)
				if err := s.db.View(func(tx *bolt.Tx) error {
					v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id))
					if err = json.Unmarshal(v, &inf); err != nil {
						log.Println("Invoice error for talent agency invoice", err)
						return err
					}

					v = tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(inf.AgencyId))
					if err = json.Unmarshal(v, &ag); err != nil {
						log.Println("Invoice error for talent agency invoice", err)
						return err
					}

					return nil
				}); err != nil {
					log.Println("Invoice error for talent agency invoice", err)
					continue
				}

				agencyFee = getTalentAgencyFee(s, inf.AgencyId)
				if agencyFee == 0 {
					log.Println("error retrieving agency fee for", inf.Id)
					continue
				}

				formatted := fmt.Sprintf(
					talentAgencyInvoiceFormat,
					ag.Name,
					ag.Id,
					infData.Payout*agencyFee,
				)
				log.Println(formatted)
			}
		}

		// TRANSFER PROCESS TO NEW MONTH
		// - We wil now add fresh deals for the new month
		// - Leftover budget from last month will be trans
		// Create a new budget key (if there isn't already one)
		// do a put on all the active campaigns in the system
		// flush all unassigned deals

		if err := s.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return err
				}

				if cmp.Active && cmp.Budget > 0 {
					// Add fresh deals for this month
					addDealsToCampaign(cmp)

					if err = saveCampaign(tx, cmp, s); err != nil {
						log.Println("Error saving campaign for billing", err)
						return
					}

					// This will carry over any left over spendable too
					// It will also look to check if there's a pending (lowered)
					// budget that was saved to db last month.. and that should be
					// used now
					var (
						leftover, pending float32
					)
					store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, budget.GetLastMonthBudgetKey())
					if err == nil && store != nil {
						leftover = store.Spendable
						pending = store.Pending
					} else {
						log.Println("Last months store not found for", cmp.Id)
					}

					// Create their budget key for this month in the DB
					// NOTE: last month's leftover spendable will be carried over
					dspFee, exchangeFee := getAdvertiserFeesFromTx(tx, s.Cfg, cmp.AdvertiserId)
					if err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, cmp, leftover, pending, dspFee, exchangeFee, true); err != nil {
						log.Println("Error creating budget key!", err)
						return err
					}
				}
				return
			})
			return nil
		}); err != nil {
			return err
		}

		return budget.UpdateLastBill(s.budgetDb, s.Cfg)
	}
	return nil
}
