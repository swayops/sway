package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
)

type BillNotify struct {
	ID     string
	Name   string
	Amount float64
}

func billingNotify(s *Server) error {
	if !isNotificationDay() {
		return nil
	}

	// Lets check if its 5 days before the first!
	notify := make(map[string][]*BillNotify)
	if err := s.db.Update(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &common.Campaign{}
			if err := json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return err
			}

			// Lets make sure this campaign has an active advertiser, active agency,
			// is set to on, is approved and has a budget!
			if !cmp.Status {
				return nil
			}

			if cmp.Approved == 0 {
				return nil
			}

			if cmp.Budget == 0 {
				return nil
			}

			var (
				ag  *auth.AdAgency
				adv *auth.Advertiser
			)

			if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
				return nil
			}

			if !ag.Status {
				return nil
			}

			if adv = s.auth.GetAdvertiser(cmp.AdvertiserId); adv == nil {
				log.Println("Could not find advertiser!", cmp.AgencyId)
				return nil
			}

			if !adv.Status {
				log.Println("Advertiser is off!", cmp.AdvertiserId)
				return nil
			}

			// Lets make sure they have an active subscription!
			allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, cmp)
			if err != nil {
				s.Alert("Stripe subscription lookup error for "+adv.ID, err)
				return nil
			}

			if !allowed && 1 == 2 {
				log.Println("Subscription is now off", adv.ID)
				return nil
			}

			if ag.IsIO {
				log.Println("Agency is IO", ag.ID)
				return nil
			}

			// This functionality carry over any left over spendable too
			// It will also look to check if there's a pending (lowered)
			// budget that was saved to db last month.. and that should be
			// used now
			var (
				pending float64
			)

			// Get store for this month since we're 5 days pre-billing
			store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
			if err == nil && store != nil {
				pending = store.Pending
			} else {
				log.Println("This months store not found for", cmp.Id)
			}

			billed := cmp.Budget
			if pending > 0 {
				// Mimics logic in CreateBudgetKey
				billed = pending
			}

			notify[cmp.AdvertiserId] = append(notify[cmp.AdvertiserId], &BillNotify{ID: cmp.Id, Name: cmp.Name, Amount: billed})

			return
		})
		return nil
	}); err != nil {
		return err
	}

	for advID, v := range notify {
		user := s.auth.GetUser(advID)
		if user == nil || user.Advertiser == nil {
			log.Println("Missing advertiser", user.ID)
			continue
		}

		if len(v) == 0 {
			log.Println("No campaigns to email about", user.ID)
			continue
		}

		if s.Cfg.ReplyMailClient() != nil {
			email := templates.NotifyBillingEmail.Render(map[string]interface{}{"Name": user.Name, "campaign": v})
			resp, err := s.Cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Sway Billing Notification for Advertiser "+user.Name), user.Email, user.Name,
				[]string{""})
			if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
				s.Alert("Failed to mail advertiser about billing notification", err)
			} else {
				if err := s.Cfg.Loggers.Log("email", map[string]interface{}{
					"tag": "billing notification",
					"id":  user.ID,
				}); err != nil {
					log.Println("Failed to log out of perks notify email!", user.ID)
				}
			}
		}
	}
	return nil
}

func daysInMonth(year int, month time.Month) int {
	t := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	return t.AddDate(0, 0, -1).Day()
}

func isNotificationDay() bool {
	// Checks to see if there are 5 days left until billing
	now := time.Now().UTC()
	days := daysInMonth(now.Year(), now.Month())
	daysUntilEnd := days - now.Day()

	return daysUntilEnd == 5
}
