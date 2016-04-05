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
	"github.com/swayops/sway/misc"
)

func newSwayEngine(srv *Server) error {
	if err := run(srv, true); err != nil {
		log.Println("Err running engine", err)
		return err
	}

	// Update social media profiles every X hours
	ticker := time.NewTicker(srv.Cfg.EngineUpdate * time.Hour)
	go func() {
		for range ticker.C {
			if err := run(srv, false); err != nil {
				log.Println("Err running engine", err)
			}
		}
	}()

	return nil
}

func run(srv *Server, skipWait bool) error {
	// Update all social media stats/completed deal stats
	if err := updateStats(srv, skipWait); err != nil {
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

	// Will run the first of every month to
	// refresh our budget dbs, budgets,
	// and send out invoices
	if err := billing(srv); err != nil {
		// Insert a file informant check
		log.Println("Err with billig!", err)
		return err
	}

	return nil
}

func updateStats(s *Server, skipWait bool) error {
	_, activeCampaigns := getAllActiveCampaigns(s)
	influencers := getAllInfluencers(s)

	// Traverses all influencers and updates their social media stats
	for _, inf := range influencers {
		// Influencer not updated if they have been updated
		// within the last s.Cfg.InfluencerTTL hours
		if err := inf.UpdateAll(s.Cfg); err != nil {
			return err
		}

		if !skipWait {
			// Inserting a request interval so we don't hit our API
			// limits with platforms!
			time.Sleep(s.Cfg.StatsInterval * time.Second)
		}

		// Update data for all completed deal posts
		if err := inf.UpdateCompletedDeals(s.Cfg, activeCampaigns); err != nil {
			return err
		}

		if !skipWait {
			time.Sleep(s.Cfg.StatsInterval * time.Second)
		}

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

	activeCampaigns, _ := getAllActiveCampaigns(s)
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

			store = budget.AdjustStore(store, deal)
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
		if len(store) == 0 || err != nil {
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
			for id, inf := range data.Influencers {
				var (
					g influencer.Influencer
					v []byte
				)
				if err := s.db.View(func(tx *bolt.Tx) error {
					v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id))
					return nil
				}); err != nil {
					log.Println("Invoice error for influencer", err)
					continue
				}

				if err = json.Unmarshal(v, &g); err != nil {
					log.Println("Invoice error for influencer", err)
					continue
				}

				formatted := fmt.Sprintf(
					infInvoiceFormat,
					g.Name,
					id,
					inf.Payout*(1-budget.TalentAgencyFee),
				)
				log.Println(formatted)
			}
		}

		// Talent Agency Invoice
		log.Println("Talent Agency Invoice")
		for _, data := range store {
			for id, data := range data.Influencers {
				var (
					inf influencer.Influencer
					v   []byte
				)
				if err := s.db.View(func(tx *bolt.Tx) error {
					v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id))
					return nil
				}); err != nil {
					log.Println("Invoice error for talent agency invoice", err)
					continue
				}

				if err = json.Unmarshal(v, &inf); err != nil {
					log.Println("Invoice error for talent agency invoice", err)
					continue
				}

				var (
					g common.TalentAgency
				)
				if err := s.db.View(func(tx *bolt.Tx) error {
					v = tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(inf.AgencyId))
					return nil
				}); err != nil {
					log.Println("Invoice error for talent agency invoice", err)
					continue
				}

				if err = json.Unmarshal(v, &g); err != nil {
					log.Println("Invoice error for talent agency invoice", err)
					continue
				}

				formatted := fmt.Sprintf(
					talentAgencyInvoiceFormat,
					g.Name,
					g.Id,
					data.Payout*budget.TalentAgencyFee,
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

		var b []byte
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

					if b, err = json.Marshal(cmp); err != nil {
						return
					}

					if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b); err != nil {
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
