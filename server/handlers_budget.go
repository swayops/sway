package server

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/swipe"
)

func getCampaignStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		misc.WriteJSON(c, 200, s.Campaigns.GetStore())
	}
}

func getBudgetInfo(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrCampaign.Error()))
			return
		}

		budgetStore, err := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || budgetStore == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, budgetStore)
	}
}

func getStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			store map[string]*budget.Store
		)
		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			store, err = budget.GetStore(tx, s.Cfg)
			if err != nil {
				return err
			}
			return nil
		}); err != nil || len(store) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		if c.Query("active") == "1" {
			filteredStore := make(map[string]*budget.Store)
			for campaignID, val := range store {
				if _, ok := s.Campaigns.Get(campaignID); ok {
					filteredStore[campaignID] = val
				}
			}
			misc.WriteJSON(c, 200, filteredStore)
		} else {
			misc.WriteJSON(c, 200, store)
		}
	}
}

type TmpPending struct {
	Budget       float64 `json:"budget,omitempty"`
	AvailBudget  float64 `json:"availBudget,omitempty"`
	BookedBudget float64 `json:"bookedBudget,omitempty"`
}

func getBudgetSnapshot(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			store map[string]*budget.Store
		)
		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			store, err = budget.GetStore(tx, s.Cfg)
			if err != nil {
				return err
			}
			return nil
		}); err != nil || len(store) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		filteredStore := make(map[string]*TmpPending)
		for campaignID, val := range store {
			cmp := common.GetCampaign(campaignID, s.db, s.Cfg)
			if cmp != nil {
				pendingSpend, _ := cmp.GetPendingDetails()
				filteredStore[campaignID] = &TmpPending{
					Budget:       cmp.Budget,
					AvailBudget:  val.Spendable - pendingSpend,
					BookedBudget: pendingSpend + val.Spent,
				}
			}
		}
		misc.WriteJSON(c, 200, filteredStore)
	}
}

type BillingInfo struct {
	ID              string           `json:"id,omitempty"`
	ActiveBalance   float64          `json:"activeBalance,omitempty"`
	InactiveBalance float64          `json:"inactiveBalance,omitempty"`
	CreditCard      *swipe.CC        `json:"cc,omitempty"`
	History         []*swipe.History `json:"history,omitempty"`
}

func getBillingInfo(s *Server) gin.HandlerFunc {
	// Retrieves all billing info for the advertiser
	return func(c *gin.Context) {
		user := s.auth.GetUser(c.Param("id"))
		if user == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		adv := user.Advertiser
		if adv == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		var (
			info BillingInfo
			err  error
		)

		if adv.Customer == "" {
			misc.WriteJSON(c, 200, info)
			return
		}

		var history []*swipe.History
		if adv.Customer != "" {
			history = swipe.GetBillingHistory(adv.Customer, user.Email, s.Cfg.Sandbox)
		}

		info.ID = adv.Customer
		info.CreditCard, err = swipe.GetCleanCreditCard(adv.Customer)
		if err != nil {
			misc.WriteJSON(c, 200, misc.StatusErr(err.Error()))
			return
		}
		info.History = history

		s.db.View(func(tx *bolt.Tx) error {
			info.InactiveBalance = budget.GetBalance(c.Param("id"), tx, s.Cfg)
			return nil
		})

		// Get all campaigns for this advertiser
		var campaigns []string
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == adv.ID {
					// No need to display massive deal set
					campaigns = append(campaigns, cmp.Id)
				}
				return
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		// Add up all spent and spendable values for the advertiser to
		// determine active budget
		for _, cmp := range campaigns {
			budg, err := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp, adv.ID)
			if err != nil || budg == nil {
				log.Println("Err retrieving budget", cmp)
				continue
			}

			info.ActiveBalance += budg.Spendable + budg.Spent
		}

		misc.WriteJSON(c, 200, info)
	}
}

func assignLikelyEarnings(s *Server) gin.HandlerFunc {
	// Handler to port over currently active deals to have
	// LikelyEarnings stored (since that's stored via the
	// assignDeal function)
	return func(c *gin.Context) {
		for _, inf := range s.auth.Influencers.GetAll() {
			for _, deal := range inf.ActiveDeals {
				if deal.LikelyEarnings == 0 {
					cmp := common.GetCampaign(deal.CampaignId, s.db, s.Cfg)
					if cmp == nil {
						log.Println("campaign not found")
						continue
					}
					maxYield := influencer.GetMaxYield(cmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)
					_, _, _, infPayout := budget.GetMargins(maxYield, -1, -1, -1)
					deal.LikelyEarnings = misc.TruncateFloat(infPayout, 2)
				}
			}
			if len(inf.ActiveDeals) > 0 {
				saveAllActiveDeals(s, inf)
			}
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

func getBalance(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var balance float64
		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			balance = budget.GetBalance(c.Param("id"), tx, s.Cfg)
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}
		misc.WriteJSON(c, 200, balance)
	}
}

type TargetYield struct {
	Min float64 `json:"min,omitempty"`
	Max float64 `json:"max,omitempty"`
}

func getTargetYield(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(fmt.Sprintf("Failed for campaign")))
			return
		}

		store, err := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || store == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(fmt.Sprintf("Failed for store")))
			return
		}

		min, max := cmp.GetTargetYield(store.Spendable)
		misc.WriteJSON(c, 200, &TargetYield{Min: min, Max: max})
	}
}
