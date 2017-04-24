package server

import (
	"fmt"
	"log"
	"time"

	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

type BillNotify struct {
	ID     string
	Name   string
	Amount float64
}

func billing(s *Server) error {
	// if !isNotificationDay() {
	// 	return nil
	// }

	// Iterate over all active campaigns
	cmps := s.Campaigns.GetStore()
	if len(cmps) == 0 {
		s.Alert("Did not run billing. Campaign store empty!", ErrWait)
		return ErrWait
	}

	for campaignID, cmp := range cmps {
		if !cmp.Status || cmp.Approved == 0 || cmp.Budget == 0 {
			continue
		}

		var (
			ag  *auth.AdAgency
			adv *auth.Advertiser
		)

		if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
			continue
		}

		if !ag.Status {
			continue
		}

		if adv = s.auth.GetAdvertiser(cmp.AdvertiserId); adv == nil {
			log.Println("Could not find advertiser!", cmp.AgencyId)
			continue
		}

		if !adv.Status {
			log.Println("Advertiser is off!", cmp.AdvertiserId)
			continue
		}

		// Lets make sure they have an active subscription!
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, cmp)
		if err != nil {
			s.Alert("Stripe subscription lookup error for "+adv.ID, err)
			continue
		}

		if !allowed {
			log.Println("Subscription is now off", adv.ID)
			continue
		}

		// This campaign's agency, advertiser, and subscription are all active!
		// Lets see if we should bill!
		store, err := budget.GetAdvertiserStore(srv.db, srv.Cfg, cmp.AdvertiserId)
		if err != nil || store == nil {
			log.Println("Error getting advertiser store for", cmp.AdvertiserId)
			continue
		}

		cmpStore, ok := store[cmp.Id]
		if !ok {
			log.Println("Error getting campaign store for", cmp.Id)
			continue
		}

		// If billing date isn't within the last day.. skip
		if !misc.WithinLast(cmpStore.NextBill, 24) {
			// If we are exactly 5 days from their billing date.. lets notify!
			if misc.WithinHours(cmpStore.NextBill, 5*24, 6*24) {
				notify[cmp.AdvertiserId] = append(notify[cmp.AdvertiserId], &BillNotify{ID: cmp.Id, Name: cmp.Name, Amount: billed})
			}

			continue
		}

		if err = budget.RemoteBill(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId, ag.IsIO); err != nil {
			s.Alert("Error when running billing for "+cmp.Id, err)
			continue
		}
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
			for _, admin := range mailingList {
				// Email admins as well
				s.Cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Sway Billing Notification for Advertiser "+user.Name), admin, user.Name,
					[]string{""})
			}
		}
	}
	return nil
}

func daysInMonth(year int, month time.Month) int {
	t := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	return t.AddDate(0, 0, -1).Day()
}

// func isNotificationDay() bool {
// 	// Checks to see if there are 5 days left until billing
// 	now := time.Now().UTC()
// 	days := daysInMonth(now.Year(), now.Month())
// 	daysUntilEnd := days - now.Day()

// 	return daysUntilEnd == 5
// }
