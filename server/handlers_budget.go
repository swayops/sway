package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/missionMeteora/mandrill"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/swipe"
)

func getCampaignStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, s.Campaigns.GetStore())
	}
}

func getBudgetInfo(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, c.Param("id"), "")
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, store)
	}
}

func getStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, err := budget.GetStore(s.budgetDb, s.Cfg, "")
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if c.Query("active") == "1" {
			filteredStore := make(map[string]*budget.Store)
			for campaignID, val := range store {
				if _, ok := s.Campaigns.Get(campaignID); ok {
					filteredStore[campaignID] = val
				}
			}
			c.JSON(200, filteredStore)
		} else {
			c.JSON(200, store)
		}
	}
}

func getLastMonthsStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, err := budget.GetStore(s.budgetDb, s.Cfg, budget.GetLastMonthBudgetKey())
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, store)
	}
}

type TmpPending struct {
	Budget       float64 `json:"budget,omitempty"`
	Spendable    float64 `json:"spendable,omitempty"`
	PendingSpend float64 `json:"pendingSpend,omitempty"`
	Spent        float64 `json:"spent,omitempty"`
}

func getBudgetSnapshot(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, err := budget.GetStore(s.budgetDb, s.Cfg, "")
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		filteredStore := make(map[string]*TmpPending)
		for campaignID, val := range store {
			if cmp, ok := s.Campaigns.Get(campaignID); ok {
				pendingSpend, _ := cmp.GetPendingDetails()
				filteredStore[campaignID] = &TmpPending{
					Budget:       cmp.Budget,
					Spendable:    val.Spendable,
					PendingSpend: pendingSpend,
					Spent:        val.Spent,
				}
			}
		}
		c.JSON(200, filteredStore)
	}
}

const (
	cmpInvoiceFormat          = "Campaign ID: %s, Email: test@sway.com, Phone: 123456789, Spent: %f, DSPFee: %f, ExchangeFee: %f, Total: %f"
	talentAgencyInvoiceFormat = "Agency ID: %s, Email: test@sway.com, Payout: %f, Influencer ID: %s, Campaign ID: %s, Deal ID: %s"
)

var (
	ErrBilling    = "There was an error running billing!"
	ErrEmptyStore = "Empty store when billing!"
)

func runBilling(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().UTC()
		if now.Day() != 1 && c.Query("force") != "1" {
			// Can only run billing on the first of the month!
			c.JSON(500, misc.StatusErr("Cannot run billing today!"))
			return
		}

		if !isSecureAdmin(c, s) {
			return
		}

		key := budget.GetLastMonthBudgetKey()
		dbg := c.Query("dbg") == "1"
		if dbg {
			// For dbg scenario, we overwrite the current
			// month's values
			key = budget.GetCurrentBudgetKey()
		}

		// Now that it's a new month.. get last month's budget store
		store, err := budget.GetStore(s.budgetDb, s.Cfg, key)
		if err != nil || len(store) == 0 {
			// Insert file informant check
			c.JSON(500, misc.StatusErr(ErrEmptyStore))
			return
		}

		agencyXf := misc.NewXLSXFile(s.Cfg.JsonXlsxPath)
		agencySheets := make(map[string]*misc.Sheet)

		// Advertiser Agency Invoice
		for cId, data := range store {
			var (
				emails string
				user   *auth.User
			)

			cmp := common.GetCampaign(cId, s.db, s.Cfg)
			if cmp == nil {
				c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for campaign, %s", cId)))
				return
			}

			user = s.auth.GetUser(cmp.AdvertiserId)
			if user != nil {
				emails = user.Email
			}

			advertiser := user.Advertiser
			if advertiser == nil {
				c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for advertiser, %s", cmp.AdvertiserId)))
				return
			}

			adAgency := s.auth.GetAdAgency(advertiser.AgencyID)
			if adAgency == nil {
				c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for ad agency, %s", cmp.AgencyId)))
				return
			}

			if adAgency.ID == auth.SwayOpsAdAgencyID {
				// Don't need any reports for SwayOps.. we pocket it all
				// because it's IO
				continue
			}

			// If an advertiser spent money they weren't charged for
			// send their asses an invoice
			invoiceDelta := data.GetDelta()
			if invoiceDelta == 0 {
				continue
			}

			// AGENCY INVOICE!
			// Don't add email for sway ad agency
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
				misc.TruncateFloat(invoiceDelta, 2),
			)

		}

		files := []string{}
		if len(agencySheets) > 0 {
			fName := fmt.Sprintf("%s-agency.xlsx", key)
			location := filepath.Join(s.Cfg.LogsPath, "invoices", fName)

			fo, err := os.Create(location)
			if err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if _, err := agencyXf.WriteTo(fo); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if err := fo.Close(); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
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
				if dbg {
					month = 0
				}
				if money := d.GetMonthStats(month); money != nil {
					talentAgency := s.auth.GetTalentAgency(inf.AgencyId)
					if talentAgency == nil {
						c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for talent agency, %s", inf.AgencyId)))
						return
					}

					user := s.auth.GetUser(talentAgency.ID)
					if user == nil {
						c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for user, %s", talentAgency.ID)))
						return
					}

					if money.AgencyId != talentAgency.ID {
						continue
					}

					cmp := common.GetCampaign(d.CampaignId, s.db, s.Cfg)
					if cmp == nil {
						c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for campaign, %s", d.CampaignId)))
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
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if _, err := talentXf.WriteTo(tvo); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if err := tvo.Close(); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
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
			_, err = s.Cfg.MailClient().SendMessageWithAttachments(fmt.Sprintf("Invoices for %s are attached!", key), fmt.Sprintf("%s Invoices", key), "shahzil@swayops.com", "Sway", nil, attachments)
			if err != nil {
				log.Println("Failed to email invoice!")
			}

			_, err = s.Cfg.MailClient().SendMessageWithAttachments(fmt.Sprintf("Invoices for %s are attached!", key), fmt.Sprintf("%s Invoices", key), "nick@swayops.com", "Sway", nil, attachments)
			if err != nil {
				log.Println("Failed to email invoice!")
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

				// Lets make sure this campaign has an active advertiser, active agency,
				// is set to on, is approved and has a budget!
				if !cmp.Status {
					if !s.Cfg.Sandbox {
						log.Println("Campaign is off", cmp.Id)
					}
					return nil
				}

				if cmp.Approved == 0 {
					log.Println("Campaign is not approved", cmp.Id)
					return nil
				}

				if cmp.Budget == 0 && !cmp.IsProductBasedBudget() {
					log.Println("Campaign has no budget", cmp.Budget)
					return nil
				}

				var (
					ag  *auth.AdAgency
					adv *auth.Advertiser
				)

				if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
					log.Println("Could not find ad agency!", cmp.AgencyId)
					return nil
				}

				if !ag.Status {
					log.Println("Agency is off!", cmp.AgencyId)
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

				if !allowed {
					log.Println("Subscription is now off", adv.ID)
					return nil
				}

				// This functionality carry over any left over spendable too
				// It will also look to check if there's a pending (lowered)
				// budget that was saved to db last month.. and that should be
				// used now
				var (
					leftover, pending float64
				)

				store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, key)
				if err == nil && store != nil {
					leftover = store.Spendable
					pending = store.Pending
				} else {
					log.Println("Last months store not found for", cmp.Id)
				}

				// Create their budget key for this month in the DB
				// NOTE: last month's leftover spendable will be carried over
				var spendable float64
				if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, cmp, leftover, pending, true, ag.IsIO, adv.Customer); err != nil {
					s.Alert("Error initializing budget key while billing for "+cmp.Id, err)
					// Don't return because an agency that switched from IO to CC that has
					// advertisers with no CC will always error here.. just alert!
					return nil
				}

				// Add fresh deals for this month
				addDealsToCampaign(cmp, s, tx, spendable)

				if err = saveCampaign(tx, cmp, s); err != nil {
					log.Println("Error saving campaign for billing", err)
					return err
				}

				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(ErrBilling))
			return
		}
		c.JSON(200, misc.StatusOK(""))
	}
}

func creditValue(s *Server) gin.HandlerFunc {
	// Credits a campaign with a certain value (determined by query param
	// whether the campaign should be charged or not)
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		cid := c.Param("campaignId")
		if cid == "" {
			c.JSON(500, misc.StatusErr("invalid campaign id"))
			return
		}

		cmp := common.GetCampaign(cid, s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, ErrCampaign)
			return
		}

		value, err := strconv.ParseFloat(c.Param("value"), 64)
		if err != nil || value == 0 {
			c.JSON(500, misc.StatusErr("invalid value"))
			return
		}

		credit, err := strconv.ParseBool(c.Param("credit"))
		if err != nil {
			c.JSON(500, misc.StatusErr("invalid credit"))
			return
		}

		// Lets make sure this campaign has an active advertiser, active agency,
		// is set to on, is approved and has a budget!
		if !cmp.Status {
			c.JSON(500, misc.StatusErr("campaign is off"))
			return
		}

		if cmp.Approved == 0 {
			c.JSON(500, misc.StatusErr("Campaign is not approved "+cmp.Id))
			return
		}

		if cmp.Budget == 0 {
			c.JSON(500, misc.StatusErr("Campaign has no budget "+cmp.Id))
			return
		}

		var (
			ag  *auth.AdAgency
			adv *auth.Advertiser
		)

		if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
			c.JSON(500, misc.StatusErr("invalid ad agency"))
			return
		}

		if !ag.Status {
			c.JSON(500, misc.StatusErr("invalid ad agency"))
			return
		}

		if adv = s.auth.GetAdvertiser(cmp.AdvertiserId); adv == nil {
			c.JSON(500, misc.StatusErr("invalid advertiser"))
			return
		}

		if !adv.Status {
			c.JSON(500, misc.StatusErr("invalid advertiser"))
			return
		}

		// Lets make sure they have an active subscription!
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, cmp)
		if err != nil {
			s.Alert("Stripe subscription lookup error for "+adv.ID, err)
			c.JSON(500, misc.StatusErr("invalid susbcription"))
			return
		}

		if !allowed {
			c.JSON(500, misc.StatusErr("invalid susbcription"))
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
			if err == nil && store != nil {
				// This campaign has a budget for this month! just charge budget

				// IsIO replaced by incoming CREDIT value so the admin can decide whether
				// they want to charge or not
				if err := budget.Credit(s.budgetDb, s.Cfg, cmp, credit, adv.Customer, value); err != nil {
					s.Alert("Error charging budget key while billing for "+cmp.Id, err)
					return err
				}

				// We just charged for budget so lets add deals for that
				addDealsToCampaign(cmp, s, tx, cmp.Budget)
			} else {
				// This campaign does not have a budget for this month. Create key!

				var spendable float64
				// Sending pending as VALUE so that the client gets credited/charged with the
				// incoming value

				// IsIO replaced by incoming CREDIT value so the admin can decide whether
				// they want to charge or not
				if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, cmp, 0, value, true, credit, adv.Customer); err != nil {
					s.Alert("Error initializing budget key while billing for "+cmp.Id, err)
					return err
				}

				// Add fresh deals for this month
				addDealsToCampaign(cmp, s, tx, spendable)
			}

			if err = saveCampaign(tx, cmp, s); err != nil {
				log.Println("Error saving campaign for billing", err)
				return err
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, misc.StatusOK(""))
	}
}

func transferSpendable(s *Server) gin.HandlerFunc {
	// Transfers spendable from last month to this month
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("campaignId"), s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, ErrCampaign)
			return
		}

		if err := budget.TransferSpendable(s.budgetDb, s.Cfg, cmp); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

func getProratedBudget(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, err := strconv.ParseFloat(c.Param("budget"), 64)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, gin.H{"budget": budget.GetProratedBudget(value)})
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
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		adv := user.Advertiser
		if adv == nil {
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		var (
			info BillingInfo
			err  error
		)

		if adv.Customer == "" {
			c.JSON(200, info)
			return
		}

		var history []*swipe.History
		if adv.Customer != "" {
			history = swipe.GetBillingHistory(adv.Customer, user.Email, s.Cfg.Sandbox)
		}

		info.ID = adv.Customer
		info.CreditCard, err = swipe.GetCleanCreditCard(adv.Customer)
		if err != nil {
			c.JSON(200, misc.StatusErr(err.Error()))
			return
		}
		info.History = history

		s.budgetDb.View(func(tx *bolt.Tx) error {
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
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		// Add up all spent and spendable values for the advertiser to
		// determine active budget
		for _, cmp := range campaigns {
			budg, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp, "")
			if err != nil {
				log.Println("Err retrieving budget", cmp)
				continue
			}

			info.ActiveBalance += budg.Spendable + budg.Spent
		}

		c.JSON(200, info)
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

		c.JSON(200, misc.StatusOK(""))
	}
}

func getBalance(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var balance float64
		if err := s.budgetDb.View(func(tx *bolt.Tx) (err error) {
			balance = budget.GetBalance(c.Param("id"), tx, s.Cfg)
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, balance)
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
			c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for campaign")))
			return
		}

		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
		if store == nil || err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		min, max := cmp.GetTargetYield(store.Spendable)
		c.JSON(200, &TargetYield{Min: min, Max: max})
	}
}
