package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"

	"github.com/gin-gonic/gin"
)

///////// Campaigns /////////
var DEFAULT_IMAGES = []string{
	"default_1.jpg",
	"default_2.jpg",
	"default_3.jpg",
	"default_4.jpg",
	"default_5.jpg",
	"default_6.jpg",
}

func postCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			cuser = auth.GetCtxUser(c)
			cmp   common.Campaign
			err   error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if !cmp.Male && !cmp.Female {
			c.JSON(400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		// Lets make sure this is a valid advertiser
		adv := s.auth.GetAdvertiser(cmp.AdvertiserId)
		if adv == nil {
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		if cuser.Admin { // if user is admin, they have to pass us an advID
			if cuser = s.auth.GetUser(cmp.AdvertiserId); cuser == nil || cuser.Advertiser == nil {
				c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
				return
			}
		} else if cuser.AdAgency != nil { // if user is an ad agency, they have to pass an advID that *they* own.
			agID := cuser.ID
			if cuser = s.auth.GetUser(cmp.AdvertiserId); cuser == nil || cuser.ParentID != agID || cuser.Advertiser == nil {
				c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
				return
			}
		}

		// cuser is always an advertiser
		cmp.AdvertiserId, cmp.AgencyId, cmp.Company = cuser.ID, cuser.ParentID, cuser.Name
		if !cmp.Twitter && !cmp.Facebook && !cmp.Instagram && !cmp.YouTube {
			c.JSON(400, misc.StatusErr("Please target atleast one social network"))
			return
		}

		if len(cmp.Tags) == 0 && cmp.Mention == "" && cmp.Link == "" {
			c.JSON(400, misc.StatusErr("Please provide a required tag, mention or link"))
			return
		}

		for _, g := range cmp.Geos {
			if !geo.IsValidGeoTarget(g) {
				c.JSON(400, misc.StatusErr("Please provide valid geo targets!"))
				return
			}
		}

		for i, ht := range cmp.Tags {
			cmp.Tags[i] = misc.SanitizeHash(ht)
		}

		cmp.Link = sanitizeURL(cmp.Link)
		cmp.Mention = sanitizeMention(cmp.Mention)
		cmp.Categories = common.LowerSlice(cmp.Categories)
		cmp.Keywords = common.LowerSlice(cmp.Keywords)
		cmp.Whitelist = common.TrimEmails(cmp.Whitelist)

		// Copy the plan from the Advertiser
		cmp.Plan = adv.Plan

		if len(adv.Blacklist) > 0 {
			// Blacklist is always set at the advertiser level using content feed bad!
			cmp.Blacklist = adv.Blacklist
		}

		if cmp.Perks != nil && cmp.Perks.Type != 1 && cmp.Perks.Type != 2 {
			c.JSON(400, misc.StatusErr("Invalid perk type. Must be 1 (Product) or 2 (Coupon)"))
			return
		}

		if cmp.Perks != nil && cmp.Perks.IsCoupon() {
			if len(cmp.Perks.Codes) == 0 {
				c.JSON(400, misc.StatusErr("Please provide coupon codes"))
				return
			}

			if cmp.Perks.Instructions == "" {
				c.JSON(400, misc.StatusErr("Please provide coupon instructions"))
				return
			}

			// Set count internally depending on number of coupon codes passed
			cmp.Perks.Count = len(cmp.Perks.Codes)
		}

		// Allowing $0 budgets for product-based campaigns!
		if cmp.Budget < 150 && cmp.Perks == nil {
			// This is NOT a budget based campaign OR a product based campaign!
			c.JSON(400, misc.StatusErr("Please provide a valid budget OR valid perks"))
			return
		}

		// Campaign always put into pending
		cmp.Approved = 0
		if c.Query("dbg") == "1" {
			cmp.Approved = int32(time.Now().Unix())
		}

		if cmp.Perks != nil && cmp.Perks.Count == 0 {
			c.JSON(400, misc.StatusErr("Please provide greater than 0 perks"))
			return
		}

		cmp.CreatedAt = time.Now().Unix()

		// Before creating the campaign.. lets make sure the plan allows for it!
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, &cmp)
		if err != nil {
			s.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
			c.JSON(400, misc.StatusErr("Current subscription plan does not allow for this campaign."))
			return
		}

		if !allowed {
			c.JSON(400, misc.StatusErr(subscriptions.GetNextPlanMsg(&cmp, adv.Plan)))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) { // have to get an id early for saveImage
			cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign)
			return
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if cmp.ImageData != "" {
			if !strings.HasPrefix(cmp.ImageData, "data:image/") {
				misc.AbortWithErr(c, 400, errors.New("Please provide a valid campaign image"))
				return
			}
			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), cmp.ImageData, cmp.Id, "", 750, 389)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			cmp.ImageURL, cmp.ImageData = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false), ""
		} else {
			cmp.ImageURL = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", DEFAULT_IMAGES[rand.Intn(len(DEFAULT_IMAGES))], false)
		}

		// We need the agency user to look at their IO status later!
		ag := s.auth.GetAdAgency(cmp.AgencyId)
		if ag == nil {
			misc.AbortWithErr(c, 400, errors.New("Please provide a valid agency ID"))
			return
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if cmp.Status {
				var spendable float64
				// Create their budget key IF the campaign is on
				// NOTE: Create budget key requires cmp.Id be set
				if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, &cmp, 0, 0, false, ag.IsIO, cuser.Advertiser.Customer); err != nil {
					s.Alert("Error initializing budget key for "+adv.Name, err)
					return
				}

				addDealsToCampaign(&cmp, s, tx, spendable)

				// Lets add to timeline!
				isCoupon := cmp.Perks != nil && cmp.Perks.IsCoupon()
				if cmp.Perks == nil || isCoupon {
					cmp.AddToTimeline(common.CAMPAIGN_APPROVAL, false, s.Cfg)
				} else {
					cmp.AddToTimeline(common.PERK_WAIT, false, s.Cfg)
				}
			}
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		go s.Notify(
			fmt.Sprintf("New campaign created %s (%s)", cmp.Name, cmp.Id),
			fmt.Sprintf("%s (%s) created a campaign for %f", adv.Name, adv.ID, cmp.Budget),
		)

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

var ErrCampaign = errors.New("Unable to retrieve campaign!")

func getCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, ErrCampaign)
			return
		}

		// This is an edge case where we need to display perk count
		// for the purpose of UI
		if cmp.Perks != nil && !s.Cfg.Sandbox && c.Query("dbg") != "1" {
			for _, d := range cmp.Deals {
				if d.Perk != nil {
					if d.Perk.Code != "" {
						cmp.Perks.Codes = append(cmp.Perks.Codes, d.Perk.Code)
					}
					cmp.Perks.Count += d.Perk.Count
				}
			}
			cmp.Perks.Count += cmp.Perks.PendingCount

		}

		if c.Query("deals") != "true" {
			// Hide deals otherwise output will get massive
			cmp.Deals = nil
		}

		c.JSON(200, cmp)
	}
}

type ManageCampaign struct {
	Image string `json:"image"`

	ID      string `json:"id"`
	Name    string `json:"name"`
	Active  bool   `json:"active"`
	Created int64  `json:"created"`

	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`

	Timeline *common.Timeline `json:"timeline"`

	Spent     float64 `json:"spent"`     // Monthly
	Remaining float64 `json:"remaining"` // Monthly
	Budget    float64 `json:"budget"`    // Monthly

	Stats *reporting.TargetStats `json:"stats"`

	Accepted  []*manageInf `json:"accepted"`
	Completed []*manageInf `json:"completed"`
}

type manageInf struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Engagements int32  `json:"engagements"`

	ImageURL   string `json:"image"`
	ProfileURL string `json:"profileUrl"`
	Followers  int64  `json:"followers"`
	PostURL    string `json:"postUrl"`
}

func getCampaignsByAdvertiser(s *Server) gin.HandlerFunc {
	// Prettifies the info because this is what Manage Campaigns
	// page on frontend uses
	return func(c *gin.Context) {
		targetAdv := c.Param("id")
		var campaigns []*ManageCampaign
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
					mCmp := &ManageCampaign{
						Image:     cmp.ImageURL,
						ID:        cmp.Id,
						Name:      cmp.Name,
						Created:   cmp.CreatedAt,
						Active:    cmp.Status,
						Twitter:   cmp.Twitter,
						Instagram: cmp.Instagram,
						YouTube:   cmp.YouTube,
						Facebook:  cmp.Facebook,
						Budget:    cmp.Budget,
					}

					if len(cmp.Timeline) > 0 {
						mCmp.Timeline = cmp.Timeline[len(cmp.Timeline)-1]
						common.SetAttribute(mCmp.Timeline)
					}

					store, _ := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
					if store != nil {
						mCmp.Spent = store.Spent
						mCmp.Remaining = store.Spendable
					}

					end := time.Now()
					start := end.AddDate(-1, 0, 0)
					mCmp.Stats, _ = reporting.GetCampaignStats(cmp.Id, s.db, s.Cfg, start, end, true)

					for _, deal := range cmp.Deals {
						inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
						if !ok {
							continue
						}

						tmpInf := &manageInf{
							ID:   inf.Id,
							Name: inf.Name,
						}

						if st := deal.TotalStats(); st != nil {
							tmpInf.Engagements = st.Likes + st.Dislikes + st.Comments + st.Shares
						}

						tmpInf.PostURL = deal.PostUrl
						if cmp.Twitter && inf.Twitter != nil {
							tmpInf.ImageURL = inf.Twitter.ProfilePicture
							tmpInf.Followers = int64(inf.Twitter.Followers)
							tmpInf.ProfileURL = inf.Twitter.GetProfileURL()
						} else if cmp.Facebook && inf.Facebook != nil {
							tmpInf.ImageURL = inf.Facebook.ProfilePicture
							tmpInf.Followers = int64(inf.Facebook.Followers)
							tmpInf.ProfileURL = inf.Facebook.GetProfileURL()
						} else if cmp.Instagram && inf.Instagram != nil {
							tmpInf.ImageURL = inf.Instagram.ProfilePicture
							tmpInf.Followers = int64(inf.Instagram.Followers)
							tmpInf.ProfileURL = inf.Instagram.GetProfileURL()
						} else if cmp.YouTube && inf.YouTube != nil {
							tmpInf.ImageURL = inf.YouTube.ProfilePicture
							tmpInf.Followers = int64(inf.YouTube.Subscribers)
							tmpInf.ProfileURL = inf.YouTube.GetProfileURL()
						}

						if deal.IsActive() {
							mCmp.Accepted = append(mCmp.Accepted, tmpInf)
						} else if deal.IsComplete() {
							mCmp.Completed = append(mCmp.Completed, tmpInf)
						}
					}
					campaigns = append(campaigns, mCmp)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, campaigns)
	}
}

func delCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Campaign
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(id)), &g)
			if err != nil {
				return
			}

			g.Status = false

			return saveCampaign(tx, g, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}

// Only these things can be changed for a campaign.. nothing else
type CampaignUpdate struct {
	Geos       []*geo.GeoRecord `json:"geos,omitempty"`
	Categories []string         `json:"categories,omitempty"`
	Keywords   []string         `json:"keywords,omitempty"`
	Status     *bool            `json:"status,omitempty"`
	Budget     *float64         `json:"budget,omitempty"`
	Male       *bool            `json:"male,omitempty"`
	Female     *bool            `json:"female,omitempty"`
	Name       *string          `json:"name,omitempty"`
	Whitelist  map[string]bool  `json:"whitelist,omitempty"`
	ImageData  string           `json:"imageData,omitempty"` // this is input-only and never saved to the db
	Task       *string          `json:"task,omitempty"`
	Perks      *common.Perk     `json:"perks,omitempty"` // NOTE: This struct only allows you to ADD to existing perks
}

func putCampaign(s *Server) gin.HandlerFunc {
	// Overrwrites any of the above campaign attributes
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
			b   []byte
		)
		cId := c.Param("id")
		if cId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid campaign ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(cId))
			return nil
		})

		if err = json.Unmarshal(b, &cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		var (
			upd CampaignUpdate
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		if upd.ImageData != "" {
			if !strings.HasPrefix(upd.ImageData, "data:image/") {
				c.JSON(400, misc.StatusErr("Please provide a valid campaign image"))
				return
			}

			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), upd.ImageData, cmp.Id, "", 750, 389)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			cmp.ImageURL, upd.ImageData = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false), ""
		}

		for _, g := range upd.Geos {
			if !geo.IsValidGeoTarget(g) {
				c.JSON(400, misc.StatusErr("Please provide valid geo targets!"))
				return
			}
		}

		if upd.Task != nil && *upd.Task != "" {
			cmp.Task = *upd.Task
		}

		if upd.Male != nil {
			cmp.Male = *upd.Male
		}

		if upd.Female != nil {
			cmp.Female = *upd.Female
		}

		if !cmp.Male && !cmp.Female {
			c.JSON(400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		if upd.Name != nil {
			if *upd.Name == "" {
				c.JSON(400, misc.StatusErr("Please provide a valid name"))
				return
			}
			cmp.Name = *upd.Name
		}

		var (
			ag  *auth.AdAgency
			adv *auth.Advertiser
		)
		if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
			c.JSON(400, misc.StatusErr("Could not find ad agency "+cmp.AgencyId))
			return
		}

		if adv = s.auth.GetAdvertiser(cmp.AdvertiserId); adv == nil {
			c.JSON(400, misc.StatusErr("Could not find advertiser "+cmp.AgencyId))
			return
		}

		cmp.Geos = upd.Geos
		cmp.Categories = common.LowerSlice(upd.Categories)
		cmp.Keywords = common.LowerSlice(upd.Keywords)

		// Copy the plan from the Advertiser
		cmp.Plan = adv.Plan

		// Before creating the campaign.. lets make sure the plan allows for it!
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, &cmp)
		if err != nil {
			s.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
			c.JSON(400, misc.StatusErr("Current subscription plan does not allow for this campaign"))
			return
		}

		if !allowed {
			c.JSON(400, misc.StatusErr(subscriptions.GetNextPlanMsg(&cmp, adv.Plan)))
			return
		}

		if upd.Budget != nil && cmp.Budget != *upd.Budget {
			// Update their budget!
			var spendable float64
			if spendable, err = budget.AdjustBudget(s.budgetDb, s.Cfg, &cmp, *upd.Budget, ag.IsIO, adv.Customer); err != nil {
				log.Println("Error creating budget key!", err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if spendable > 0 {
				s.db.Update(func(tx *bolt.Tx) error {
					addDealsToCampaign(&cmp, s, tx, spendable)
					return nil
				})
			}

			cmp.Budget = *upd.Budget
		}

		updatedWl := common.TrimEmails(upd.Whitelist)
		additions := []string{}
		for email, _ := range updatedWl {
			// If the email isn't on the old whitelist
			// lets email them since they're an addition!
			if _, ok := cmp.Whitelist[email]; !ok {
				additions = append(additions, email)
			}
		}

		cmp.Whitelist = updatedWl
		// If there are additions and the campaign is already approved..
		if len(additions) > 0 && cmp.Approved > 0 {
			go emailList(s, cmp.Id, additions)
		}

		if upd.Perks != nil && cmp.Perks != nil {
			// Only update if the campaign already has perks..
			if cmp.Perks.IsCoupon() && upd.Perks.IsCoupon() {
				// If the saved perk is a coupon.. lets add more!
				existingCoupons := cmp.Perks.Codes

				// Get all the coupons saved in the deals
				for _, d := range cmp.Deals {
					if d.Perk != nil && d.Perk.Code != "" {
						existingCoupons = append(existingCoupons, d.Perk.Code)
					}
				}

				// Only add the codes which are not already saved!
				var filteredList []string
				for _, newCouponCode := range upd.Perks.Codes {
					if !common.IsInList(existingCoupons, newCouponCode) {
						filteredList = append(filteredList, newCouponCode)
					}
				}

				dealsToAdd := len(filteredList)
				if dealsToAdd > 0 {
					cmp.Perks.Codes = append(cmp.Perks.Codes, filteredList...)
					cmp.Perks.Count += dealsToAdd

					// Add deals for perks we added
					if err = s.db.Update(func(tx *bolt.Tx) (err error) {
						addDeals(&cmp, dealsToAdd, s, tx)
						return saveCampaign(tx, &cmp, s)
					}); err != nil {
						misc.AbortWithErr(c, 500, err)
						return
					}
				}
			} else if !cmp.Perks.IsCoupon() && !upd.Perks.IsCoupon() {
				// If the saved perk is a physical product.. lets add more!
				perksInUse := cmp.Perks.Count

				// Get all the products saved in the deals
				for _, d := range cmp.Deals {
					if d.Perk != nil {
						perksInUse += d.Perk.Count
					}
				}

				if perksInUse > upd.Perks.Count {
					c.JSON(400, misc.StatusErr("Perk count can only be increased"))
					return
				}

				// Subtracting pending count because frontend's upd.Perks.Count value
				// includes pending count!
				dealsToAdd := upd.Perks.Count - perksInUse - cmp.Perks.PendingCount
				if dealsToAdd > 0 {
					// For perks.. lets put it into pending count instead of the actual
					// count
					// NOTE: We'll add deals and perk count once the addition is approved
					// by admin
					cmp.Perks.PendingCount += dealsToAdd
					// Add deals for perks we added
					if err = s.db.Update(func(tx *bolt.Tx) (err error) {
						return saveCampaign(tx, &cmp, s)
					}); err != nil {
						misc.AbortWithErr(c, 500, err)
						return
					}
				}
			}
		}

		// Save the Campaign
		var turnedOff bool
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if len(additions) > 0 && cmp.Perks == nil {
				// Add deals depending on how many whitelisted influencers were added
				// ONLY if it's a non-perk based campaign. We don't add deals for whitelists
				// for perks
				addDeals(&cmp, len(additions), s, tx)
			}

			if upd.Status == nil {
				goto END
			}

			if *upd.Status && cmp.Status != *upd.Status {
				// If the campaign has been toggled to on..
				store, _ := budget.GetBudgetInfo(s.budgetDb, s.Cfg, cmp.Id, "")
				if store == nil {
					// This means the campaign has no store.. cmp was craeted with a status of
					// off so a budget key was NOT created.. so we have to create it now!
					// NOTE: Create budget key requires cmp.Id be set

					var spendable float64
					if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, &cmp, 0, 0, false, ag.IsIO, adv.Customer); err != nil {
						s.Alert("Error initializing budget key for "+adv.Name, err)
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}

					addDealsToCampaign(&cmp, s, tx, spendable)

				} else {
					// This campaign does have a store.. so it was active sometime this month.
					// Lets just give it the spendable we must have taken from it when it turned
					// off
					err = budget.ReplenishSpendable(s.budgetDb, s.Cfg, &cmp, ag.IsIO, adv.Customer)
					if err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}

				// Campaign has been toggled to on.. so lets add to timeline
				isCoupon := cmp.Perks != nil && cmp.Perks.IsCoupon()
				if cmp.Perks == nil || isCoupon {
					if cmp.Approved == 0 {
						cmp.AddToTimeline(common.CAMPAIGN_APPROVAL, false, s.Cfg)
					} else {
						cmp.AddToTimeline(common.CAMPAIGN_START, false, s.Cfg)
					}
				} else {
					if cmp.Approved == 0 {
						cmp.AddToTimeline(common.PERK_WAIT, false, s.Cfg)
					} else {
						cmp.AddToTimeline(common.PERKS_RECEIVED, false, s.Cfg)
					}
				}
			} else if !*upd.Status && cmp.Status != *upd.Status {
				// If the status has been toggled to off..
				// Clear out the spendable from the DB and add that value to the balance
				var spendable float64
				spendable, err = budget.ClearSpendable(s.budgetDb, s.Cfg, &cmp)
				if err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}

				// If we cleared out some spendable.. lets increment the balance with it
				if spendable > 0 {
					if err = s.budgetDb.Update(func(tx *bolt.Tx) (err error) {
						return budget.IncrBalance(cmp.AdvertiserId, spendable, tx, s.Cfg)
					}); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}
				turnedOff = true

				// Since the campaign has toggled to off.. lets add to timeline
				cmp.AddToTimeline(common.CAMPAIGN_PAUSED, false, s.Cfg)
			}
			cmp.Status = *upd.Status

		END:
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if turnedOff {
			// Lets disactivate all currently ASSIGNED deals
			// and email the influencer to alert them if the campaign was turned off
			go func() {
				// Wait 15 mins before emailing
				dbg := c.Query("dbg") == "1"
				if !dbg {
					time.Sleep(15 * time.Minute)
				}
				emailStatusUpdate(s, cmp.Id, dbg)
			}()
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

func getForecast(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Gets influencer count and reach for an incoming campaign struct
		// NOTE: Ignores budget values

		var (
			cmp common.Campaign
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		influencers, reach := getForecastForCmp(s, cmp)

		if c.Query("breakdown") != "" {
			c.JSON(200, gin.H{"influencers": len(influencers), "reach": reach, "breakdown": influencers})
		} else {
			// Default to totals
			c.JSON(200, gin.H{"influencers": len(influencers), "reach": reach})
		}
	}
}

func getLatestGeo(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, inf.GetLatestGeo())
	}
}

func getPendingCampaigns(s *Server) gin.HandlerFunc {
	// Have we received the perks from advertiser? Are there any campaigns that have NOT
	// been approved by admin yet?

	// Maps to label "INBOUND" on admin frontend
	return func(c *gin.Context) {
		var campaigns []*common.Campaign
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.Approved == 0 || (cmp.Perks != nil && cmp.Perks.PendingCount > 0) {
					// Hide deals
					cmp.Deals = nil
					campaigns = append(campaigns, &cmp)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, campaigns)
	}
}

func approveCampaign(s *Server) gin.HandlerFunc {
	// Used once we have approved the campaign on admin page "INBOUND!
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
			b   []byte
		)
		cId := c.Param("id")
		if cId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid campaign ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(cId))
			return nil
		})

		if err = json.Unmarshal(b, &cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		user := s.auth.GetUser(cmp.AdvertiserId)
		if user == nil || user.Advertiser == nil {
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		if cmp.Perks != nil && cmp.Perks.PendingCount > 0 {
			// Add deals for perks we added
			if err = s.db.Update(func(tx *bolt.Tx) (err error) {
				// Add deals for pending value once we have received product!
				addDeals(&cmp, cmp.Perks.PendingCount, s, tx)

				// Empty out fields and increment perk count
				cmp.Perks.Count += cmp.Perks.PendingCount
				cmp.Perks.PendingCount = 0

				return saveCampaign(tx, &cmp, s)
			}); err != nil {
				misc.AbortWithErr(c, 500, err)
				return
			}
		}

		if !s.Cfg.Sandbox && cmp.Perks != nil && !cmp.Perks.IsCoupon() {
			// Lets let the advertiser know that we've received their product!
			if s.Cfg.ReplyMailClient() != nil {
				email := templates.NotifyPerkEmail.Render(map[string]interface{}{"Perk": cmp.Perks.Name, "Name": user.Advertiser.Name})
				resp, err := s.Cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("Your shipment has been received!"), user.Email, user.Name,
					[]string{""})
				if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
					s.Alert("Failed to mail advertiser regarding shipment", err)
				} else {
					if err := s.Cfg.Loggers.Log("email", map[string]interface{}{
						"tag": "received shipment",
						"id":  user.ID,
					}); err != nil {
						log.Println("Failed to log shipment received notify email!", user.ID)
					}
				}
			}
		}

		// Bail early if this JUST an acceptance for a perk increase!
		if cmp.Approved > 0 {
			c.JSON(200, misc.StatusOK(cmp.Id))
			return
		}

		cmp.Approved = int32(time.Now().Unix())

		// Lets add to timeline!
		isCoupon := cmp.Perks != nil && cmp.Perks.IsCoupon()
		if cmp.Perks == nil || isCoupon {
			cmp.AddToTimeline(common.CAMPAIGN_START, false, s.Cfg)
		} else {
			cmp.AddToTimeline(common.PERKS_RECEIVED, false, s.Cfg)
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Email eligible influencers now that campaign is approved!
		if len(cmp.Whitelist) > 0 {
			go func() {
				emailList(s, cmp.Id, nil)
			}()
		} else {
			go func() {
				// Wait 2 hours before emailing
				time.Sleep(2 * time.Hour)
				emailDeal(s, cmp.Id)
			}()
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

func uploadImage(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var upd UploadImage
		if err := json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		id := c.Param("id")
		if id == "" {
			c.JSON(400, misc.StatusErr("Invalid ID"))
			return
		}

		bucket := c.Param("bucket")
		filename, err := saveImageToDisk(s.Cfg.ImagesDir+bucket+"/"+id, upd.Data, id, "", 750, 389)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		var imageURL string
		if bucket == "campaign" {
			var (
				cmp common.Campaign
				b   []byte
			)
			// Save image URL in campaign
			s.db.View(func(tx *bolt.Tx) error {
				b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(id))
				return nil
			})

			if err = json.Unmarshal(b, &cmp); err != nil {
				c.JSON(400, misc.StatusErr("Error unmarshalling campaign"))
				return
			}

			imageURL = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false)
			cmp.ImageURL = imageURL

			// Save the Campaign
			if err = s.db.Update(func(tx *bolt.Tx) (err error) {
				return saveCampaign(tx, &cmp, s)
			}); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
		}
		c.JSON(200, UploadImage{ImageURL: imageURL})
	}
}
