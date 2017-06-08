package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	"github.com/missionMeteora/mandrill"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

type BillNotify struct {
	ID     string
	Name   string
	Amount float64
}

func (s *Server) billing() error {
	if isInvoiceDay() {
		generateInvoices(s)
	}

	// Iterate over all active campaigns
	cmps := s.Campaigns.GetStore()
	if len(cmps) == 0 {
		return nil
	}

	notify := make(map[string][]*BillNotify)
	for _, cmp := range cmps {
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
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, &cmp)
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

		cmpStore, err := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || cmpStore == nil {
			log.Println("Error when opening store", err)
			continue
		}

		// If billing date isn't within the last day.. skip
		if !misc.WithinLast(int32(cmpStore.NextBill), 24) {
			// If we are exactly 5 days from their billing date.. lets notify!
			if misc.WithinHours(int32(cmpStore.NextBill), 5*24, 6*24) {
				notify[cmp.AdvertiserId] = append(notify[cmp.AdvertiserId], &BillNotify{ID: cmp.Id, Name: cmp.Name, Amount: cmp.Budget})
			}
			continue
		}

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if err = budget.RemoteBill(tx, s.Cfg, &cmp, adv.Customer, ag.IsIO); err != nil {
				return err
			}
			return nil
		}); err != nil {
			s.Alert("Error when running billing for "+cmp.Id, err)
			continue
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			// Add fresh deals for this month
			addDealsToCampaign(&cmp, s, tx, cmp.Budget)
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			s.Alert("Error saving campaign "+cmp.Id, err)
			return err
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

		if s.Cfg.ReplyMailClient() != nil && !s.Cfg.Sandbox {
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

func generateInvoices(s *Server) {
	key := time.Now().UTC().Format("01-2006")
	// Get all campaigns in the system
	cmps := getAllCampaigns(s.db, s.Cfg)
	if len(cmps) == 0 {
		s.Alert("No campaigns to invoice!", nil)
		return
	}

	// Get all spend in the last month
	now := time.Now().UTC()

	// Gets first and last date of last month
	from, to := firstAndLast(now.Year(), now.Month())

	// Initialize sheets
	agencyXf := misc.NewXLSXFile(s.Cfg.JsonXlsxPath)
	agencySheets := make(map[string]*misc.Sheet)

	for _, cmp := range cmps {
		var (
			emails string
			user   *auth.User
		)

		stats, err := reporting.GetCampaignStats(cmp.Id, s.db, s.Cfg, from, to, true)
		if err != nil {
			s.Alert("Error retrieving last month's stats for invoicing", err)
			continue
		}

		user = s.auth.GetUser(cmp.AdvertiserId)
		if user != nil {
			emails = user.Email
		}

		advertiser := user.Advertiser
		if advertiser == nil {
			continue
		}

		adAgency := s.auth.GetAdAgency(advertiser.AgencyID)
		if adAgency == nil {
			continue
		}

		if !adAgency.IsIO {
			continue
		}

		if stats == nil || stats.Total == nil || stats.Total.Spent == 0 {
			// Did not spend anything!
			continue
		}

		user = s.auth.GetUser(adAgency.ID)
		if user != nil {
			if emails == "" {
				emails = user.Email
			} else {
				emails += ", " + user.Email
			}
		}

		sheet, ok := agencySheets[adAgency.ID]
		if !ok {
			sheet = agencyXf.AddSheet(fmt.Sprintf("%s (%s)", adAgency.Name, adAgency.ID))
			sheet.AddHeader(
				"Advertiser ID",
				"Advertiser Name",
				"Campaign ID",
				"Campaign Name",
				"Emails",
				"DSP Fee",
				"Exchange Fee",
				"Total Spent ($)",
			)
			agencySheets[adAgency.ID] = sheet
		}
		dspFee, exchangeFee := getAdvertiserFees(s.auth, cmp.AdvertiserId)
		sheet.AddRow(
			cmp.AdvertiserId,
			advertiser.Name,
			cmp.Id,
			cmp.Name,
			emails,
			fmt.Sprintf("%0.2f", dspFee*100)+"%",
			fmt.Sprintf("%0.2f", exchangeFee*100)+"%",
			misc.TruncateFloat(stats.Total.Spent, 2),
		)
	}

	files := []string{}
	if len(agencySheets) > 0 {
		fName := fmt.Sprintf("%s-agency.xlsx", key)
		location := filepath.Join(s.Cfg.LogsPath, "invoices", fName)

		fo, err := os.Create(location)
		if err != nil {
			s.Alert("Error creating agency xlsx", err)
			return
		}

		if _, err := agencyXf.WriteTo(fo); err != nil {
			s.Alert("Error creating agency xlsx", err)
			return
		}

		if err := fo.Close(); err != nil {
			s.Alert("Error creating agency xlsx", err)
			return
		}

		files = append(files, fName)
	}

	// Talent Agency Invoice
	talentXf := misc.NewXLSXFile(s.Cfg.JsonXlsxPath)
	talentSheets := make(map[string]*misc.Sheet)

	for _, infId := range s.auth.Influencers.GetAllIDs() {
		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			continue
		}

		for _, d := range inf.CompletedDeals {
			// Get payouts for last month since it's the first
			month := 1
			if money := d.GetMonthStats(month); money != nil {
				talentAgency := s.auth.GetTalentAgency(inf.AgencyId)
				if talentAgency == nil {
					continue
				}

				user := s.auth.GetUser(talentAgency.ID)
				if user == nil {
					continue
				}

				if money.AgencyId != talentAgency.ID {
					continue
				}

				cmp := common.GetCampaign(d.CampaignId, s.db, s.Cfg)
				if cmp == nil {
					log.Println("No campaign wtf", d.CampaignId)
					return
				}

				sheet, ok := talentSheets[talentAgency.ID]
				if !ok {
					sheet = talentXf.AddSheet(fmt.Sprintf("%s (%s)", talentAgency.Name, talentAgency.ID))

					sheet.AddHeader(
						"",
						"Influencer ID",
						"Influencer Name",
						"Campaign ID",
						"Campaign Name",
						"Agency Payout ($)",
					)
					talentSheets[talentAgency.ID] = sheet
				}
				if len(sheet.Rows) == 0 {
					sheet.AddRow(
						user.Email,
						inf.Id,
						inf.Name,
						cmp.Id,
						cmp.Name,
						misc.TruncateFloat(money.Agency, 2),
					)
				} else {
					sheet.AddRow(
						"",
						inf.Id,
						inf.Name,
						cmp.Id,
						cmp.Name,
						misc.TruncateFloat(money.Agency, 2),
					)
				}

			}
		}
	}

	if len(talentSheets) > 0 {
		fName := fmt.Sprintf("%s-talent.xlsx", key)
		location := filepath.Join(s.Cfg.LogsPath, "invoices", fName)
		tvo, err := os.Create(location)
		if err != nil {
			s.Alert("Error creating talent xlsx", err)
			return
		}

		if _, err := talentXf.WriteTo(tvo); err != nil {
			s.Alert("Error creating talent xlsx", err)
			return
		}

		if err := tvo.Close(); err != nil {
			s.Alert("Error creating talent xlsx", err)
			return
		}

		files = append(files, fName)
	}

	// Email!
	var attachments []*mandrill.MessageAttachment
	for _, fName := range files {
		f, err := os.Open(filepath.Join(s.Cfg.LogsPath, "invoices", fName))
		if err != nil {
			log.Println("Failed to open file!", fName)
			continue
		}

		att, err := mandrill.AttachmentFromReader(fName, f)
		f.Close()
		if err != nil {
			log.Println("Unable to create attachment!", err)
			f.Close()
			continue
		}
		attachments = append(attachments, att)
	}

	if len(attachments) > 0 && !s.Cfg.Sandbox {
		_, err := s.Cfg.MailClient().SendMessageWithAttachments(fmt.Sprintf("Invoices for %s are attached!", key), fmt.Sprintf("%s Invoices", key), "shahzil@swayops.com", "Sway", nil, attachments)
		if err != nil {
			log.Println("Failed to email invoice!")
		}

		_, err = s.Cfg.MailClient().SendMessageWithAttachments(fmt.Sprintf("Invoices for %s are attached!", key), fmt.Sprintf("%s Invoices", key), "nick@swayops.com", "Sway", nil, attachments)
		if err != nil {
			log.Println("Failed to email invoice!")
		}
	}

}

func firstAndLast(year int, month time.Month) (time.Time, time.Time) {
	from := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)
	return from, to
}

func isInvoiceDay() bool {
	// Checks to see if it's the first of the month!
	now := time.Now().UTC()
	return now.Day() == 1
}
