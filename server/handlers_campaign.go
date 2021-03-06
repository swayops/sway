package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"

	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
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

func delCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 404, ErrCampaign)
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			cmp.Status, cmp.Archived = false, true
			return saveCampaign(tx, cmp, s)
		}); err != nil {
			log.Printf("error: %v", err)
			misc.WriteJSON(c, 500, ErrCampaign)
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(cmp.Id))
	}
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
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if !cmp.Male && !cmp.Female {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		// Lets make sure this is a valid advertiser
		adv := s.auth.GetAdvertiser(cmp.AdvertiserId)
		if adv == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		if cuser.Admin { // if user is admin, they have to pass us an advID
			if cuser = s.auth.GetUser(cmp.AdvertiserId); cuser == nil || cuser.Advertiser == nil {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
				return
			}
		} else if cuser.AdAgency != nil { // if user is an ad agency, they have to pass an advID that *they* own.
			agID := cuser.ID
			if cuser = s.auth.GetUser(cmp.AdvertiserId); cuser == nil || cuser.ParentID != agID || cuser.Advertiser == nil {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
				return
			}
		}

		// cuser is always an advertiser
		cmp.AdvertiserId, cmp.AgencyId, cmp.Company = cuser.ID, cuser.ParentID, cuser.Name
		if !cmp.Twitter && !cmp.Facebook && !cmp.Instagram && !cmp.YouTube {
			misc.WriteJSON(c, 400, misc.StatusErr("Please target atleast one social network"))
			return
		}

		if len(cmp.Tags) == 0 && cmp.Mention == "" {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a required hashtag or mention"))
			return
		}

		for _, g := range cmp.Geos {
			if !geo.IsValidGeoTarget(g) {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide valid geo targets!"))
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

		cmp.Whitelist = common.TrimWhitelist(cmp.Whitelist)
		cmp.CampaignBlacklist = common.TrimEmails(cmp.CampaignBlacklist)
		now := time.Now().Unix()
		// Lets do a sanity check on the schedule for the whitelist
		for _, schedule := range cmp.Whitelist {
			if schedule != nil && schedule.From > 0 && schedule.To > 0 {
				if schedule.To < now {
					// Old date!
					misc.WriteJSON(c, 400, misc.StatusErr("Please enter a whitelist schedule from the future!"))
					return
				}

				if schedule.From > schedule.To {
					misc.WriteJSON(c, 400, misc.StatusErr("Schedule start date is newer than schedule end date!"))
					return
				}

				if schedule.From == schedule.To {
					misc.WriteJSON(c, 400, misc.StatusErr("Schedule start date is equal to schedule end date!"))
					return
				}
			}
		}

		// Copy the plan from the Advertiser
		cmp.Plan = adv.Plan

		if len(adv.Blacklist) > 0 {
			// Blacklist is always set at the advertiser level using content feed bad!
			cmp.AdvertiserBlacklist = adv.Blacklist
		}

		if cmp.Perks != nil && cmp.Perks.Type != 1 && cmp.Perks.Type != 2 {
			misc.WriteJSON(c, 400, misc.StatusErr("Invalid perk type. Must be 1 (Product) or 2 (Coupon)"))
			return
		}

		if cmp.Perks != nil && cmp.Perks.IsCoupon() {
			if len(cmp.Perks.Codes) == 0 {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide coupon codes"))
				return
			}

			if cmp.Perks.Instructions == "" {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide coupon instructions"))
				return
			}

			// Set count internally depending on number of coupon codes passed
			cmp.Perks.Count = len(cmp.Perks.Codes)
		}

		// Allowing $0 budgets for product-based campaigns!
		if cmp.Budget < 150 && cmp.Perks == nil {
			// This is NOT a budget based campaign OR a product based campaign!
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid budget OR valid perks"))
			return
		}

		// Campaign always put into pending
		cmp.Approved = 0
		if c.Query("dbg") == "1" {
			cmp.Approved = int32(time.Now().Unix())
		}

		if cmp.Perks != nil && cmp.Perks.Count == 0 {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide greater than 0 perks"))
			return
		}

		cmp.CreatedAt = time.Now().Unix()

		// Before creating the campaign.. lets make sure the plan allows for it!
		allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, &cmp)
		if err != nil {
			s.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
			misc.WriteJSON(c, 400, misc.StatusErr("Current subscription plan does not allow for this campaign."))
			return
		}

		if !allowed {
			misc.WriteJSON(c, 400, misc.StatusErr(subscriptions.GetNextPlanMsg(&cmp, adv.Plan)))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) { // have to get an id early for saveImage
			cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign)
			return
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		if cmp.ImageData != "" {
			if !strings.HasPrefix(cmp.ImageData, "data:image/") {
				misc.AbortWithErr(c, 400, errors.New("Please provide a valid campaign image"))
				return
			}
			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), cmp.ImageData, cmp.Id, "", 750, 389)
			if err != nil {
				misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
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
				// Create their budget key IF the campaign is on
				// NOTE: Create budget key requires cmp.Id be set
				if err = budget.Create(tx, s.Cfg, &cmp, ag.IsIO, cuser.Advertiser.Customer); err != nil {
					s.Alert("Error initializing budget key for "+adv.Name, err)
					return
				}

				addDealsToCampaign(&cmp, s, tx, cmp.Budget)

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

		misc.WriteJSON(c, 200, misc.StatusOK(cmp.Id))
	}
}

var ErrCampaign = errors.New("Unable to retrieve campaign!")

func getCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 404, ErrCampaign)
			return
		}

		if cmp.Archived && !auth.GetCtxUser(c).Admin {
			misc.WriteJSON(c, 404, ErrCampaign)
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

		misc.WriteJSON(c, 200, cmp)
	}
}

func getRejections(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("campaignId"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, ErrCampaign)
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		campaigns := common.NewCampaigns(nil)
		campaigns.SetCampaign(cmp.Id, *cmp)
		_, rejections := inf.GetAvailableDeals(campaigns, s.Audiences, s.db, "", "", nil, false, s.getTalentAgencyFee(inf.AgencyId), s.Cfg)

		misc.WriteJSON(c, 200, rejections)
	}
}

type Cycle struct {
	Matched   int `json:"matched"`
	Notified  int `json:"notified"`
	Accepted  int `json:"accepted"`
	Perks     int `json:"perks"`
	Completed int `json:"completed"`
}

func getCycle(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, ErrCampaign)
			return
		}

		_, total, _, _ := getForecastForCmp(s, *cmp, "", "", "", -1, -1)

		cycle := Cycle{
			Matched:   total,
			Notified:  len(cmp.Notifications),
			Accepted:  cmp.GetAcceptedCount(),
			Perks:     cmp.GetPerksSent(),
			Completed: cmp.GetCompletedCount(),
		}

		if cmp.Id == "30" {
			cycle.Notified += 660
		}

		misc.WriteJSON(c, 200, cycle)
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

	Archived bool `json:"archived,omitempty"`
}

type manageInf struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	EmailAddr   string `json:"emailAddress"`
	Engagements int32  `json:"engagements"`
	DealID      string `json:"dealID"`
	ImageURL    string `json:"image"`
	ProfileURL  string `json:"profileUrl"`
	Followers   int64  `json:"followers"`
	PostURL     string `json:"postUrl"`

	Submission *common.Submission `json:"submission"`
}

func getCampaignsByAdvertiser(s *Server) gin.HandlerFunc {
	// Prettifies the info because this is what Manage Campaigns
	// page on frontend uses
	return func(c *gin.Context) {
		var (
			targetAdv = c.Param("id")
			campaigns []*ManageCampaign
			isAdmin   = auth.GetCtxUser(c).Admin
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if cmp.Archived && !isAdmin {
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
						Archived:  cmp.Archived,
					}

					if len(cmp.Timeline) > 0 {
						mCmp.Timeline = cmp.Timeline[len(cmp.Timeline)-1]
						common.SetAttribute(mCmp.Timeline)
					}

					store, _ := budget.GetCampaignStore(tx, s.Cfg, cmp.Id, cmp.AdvertiserId)
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
							ID:        inf.Id,
							Name:      inf.Name,
							DealID:    deal.Id,
							EmailAddr: inf.EmailAddress,
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
							// Only append submission if it's not approved yet
							if deal.Submission != nil && !deal.Submission.Approved {
								tmpInf.Submission = deal.Submission
							}
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
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		sort.Slice(campaigns, func(i int, j int) bool {
			return campaigns[i].Created > campaigns[j].Created
		})

		misc.WriteJSON(c, 200, campaigns)
	}
}

func dirtyHack(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if cmp.Id != "21" {
					return nil
				}

				cmp.Link = "https://www.amazon.com/gp/product/B01C2EFBZU?th=1"

				return saveCampaign(tx, &cmp, s)
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

// Only these things can be changed for a campaign.. nothing else
type CampaignUpdate struct {
	Geos               []*geo.GeoRecord         `json:"geos,omitempty"`
	Categories         []string                 `json:"categories,omitempty"`
	Audiences          []string                 `json:"audiences,omitempty"`
	Keywords           []string                 `json:"keywords,omitempty"`
	Status             *bool                    `json:"status,omitempty"`
	Budget             *float64                 `json:"budget,omitempty"`
	Monthly            *bool                    `json:"monthly,omitempty"`
	TermsAndConditions *string                  `json:"terms,omitempty"`
	Male               *bool                    `json:"male,omitempty"`
	Female             *bool                    `json:"female,omitempty"`
	Name               *string                  `json:"name,omitempty"`
	Whitelist          map[string]*common.Range `json:"whitelistSchedule,omitempty"`
	ImageData          string                   `json:"imageData,omitempty"` // this is input-only and never saved to the db
	Task               *string                  `json:"task,omitempty"`
	Perks              *common.Perk             `json:"perks,omitempty"` // NOTE: This struct only allows you to ADD to existing perks
	BrandSafe          *bool                    `json:"brandSafe,omitempty"`
	RequiresSubmission *bool                    `json:"reqSub,omitempty"` // Does the advertiser require submission?
	CampaignBlacklist  map[string]bool          `json:"cmpBlacklist,omitempty"`

	FollowerTarget *common.Range      `json:"followerTarget,omitempty"`
	EngTarget      *common.Range      `json:"engTarget,omitempty"`
	PriceTarget    *common.FloatRange `json:"priceTarget,omitempty"`
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
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid campaign ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(cId))
			return nil
		})

		if err = json.Unmarshal(b, &cmp); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		var (
			upd CampaignUpdate
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		if upd.ImageData != "" {
			if !strings.HasPrefix(upd.ImageData, "data:image/") {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid campaign image"))
				return
			}

			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), upd.ImageData, cmp.Id, "", 750, 389)
			if err != nil {
				misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
				return
			}

			cmp.ImageURL, upd.ImageData = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false), ""
		}

		for _, g := range upd.Geos {
			if !geo.IsValidGeoTarget(g) {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide valid geo targets!"))
				return
			}
		}

		if upd.Task != nil && *upd.Task != "" {
			cmp.Task = *upd.Task
			// Also update task in any deals
			for _, d := range cmp.Deals {
				if d.IsActive() {
					d.Task = cmp.Task
					cmp.Deals[d.Id] = d
				}
			}
		}

		if upd.TermsAndConditions != nil && *upd.TermsAndConditions != "" {
			cmp.TermsAndConditions = *upd.TermsAndConditions
		}

		if upd.Male != nil {
			cmp.Male = *upd.Male
		}

		if upd.Female != nil {
			cmp.Female = *upd.Female
		}

		if upd.BrandSafe != nil {
			cmp.BrandSafe = *upd.BrandSafe
		}

		if upd.Monthly != nil {
			cmp.Monthly = *upd.Monthly
		}

		if upd.RequiresSubmission != nil {
			cmp.RequiresSubmission = *upd.RequiresSubmission
		}

		if upd.FollowerTarget != nil {
			cmp.FollowerTarget = upd.FollowerTarget
		}

		if !cmp.Male && !cmp.Female {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		if upd.Name != nil {
			if *upd.Name == "" {
				misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid name"))
				return
			}
			cmp.Name = *upd.Name
		}

		var (
			ag  *auth.AdAgency
			adv *auth.Advertiser
		)
		if ag = s.auth.GetAdAgency(cmp.AgencyId); ag == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Could not find ad agency "+cmp.AgencyId))
			return
		}

		if adv = s.auth.GetAdvertiser(cmp.AdvertiserId); adv == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Could not find advertiser "+cmp.AgencyId))
			return
		}

		cmp.Geos = upd.Geos
		cmp.Categories = common.LowerSlice(upd.Categories)
		cmp.Keywords = common.LowerSlice(upd.Keywords)
		cmp.Audiences = upd.Audiences
		cmp.FollowerTarget = upd.FollowerTarget
		cmp.EngTarget = upd.EngTarget
		cmp.PriceTarget = upd.PriceTarget

		// Copy the plan from the Advertiser
		cmp.Plan = adv.Plan

		// If the campaign is being toggled to off.. who cares about subscription
		if upd.Status == nil || !*upd.Status {
			// Before creating the campaign.. lets make sure the plan allows for it!
			allowed, err := subscriptions.CanCampaignRun(adv.IsSelfServe(), adv.Subscription, adv.Plan, &cmp)
			if err != nil {
				s.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
				misc.WriteJSON(c, 400, misc.StatusErr("Current subscription plan does not allow for this campaign"))
				return
			}

			if !allowed {
				misc.WriteJSON(c, 400, misc.StatusErr(subscriptions.GetNextPlanMsg(&cmp, adv.Plan)))
				return
			}
		}

		if upd.Budget != nil && cmp.Budget != *upd.Budget {
			// If the budget has been increased.. lets just
			// Update their budget!
			cmp.Budget = *upd.Budget
		}

		updatedWl := common.TrimWhitelist(upd.Whitelist)
		additions := []string{}
		for email, _ := range updatedWl {
			// If the email isn't on the old whitelist
			// lets email them since they're an addition!
			if _, ok := cmp.Whitelist[email]; !ok {
				additions = append(additions, email)
			}
		}

		cmp.Whitelist = updatedWl

		cmp.CampaignBlacklist = common.TrimEmails(upd.CampaignBlacklist)

		now := time.Now().Unix()
		// Lets do a sanity check on the schedule for the whitelist
		for _, schedule := range cmp.Whitelist {
			if schedule != nil && schedule.From > 0 && schedule.To > 0 {
				if schedule.To < now {
					// Old date!
					misc.WriteJSON(c, 400, misc.StatusErr("Please enter a whitelist schedule from the future!"))
					return
				}

				if schedule.From > schedule.To {
					misc.WriteJSON(c, 400, misc.StatusErr("Schedule start date is newer than schedule end date!"))
					return
				}

				if schedule.From == schedule.To {
					misc.WriteJSON(c, 400, misc.StatusErr("Schedule start date is equal to schedule end date!"))
					return
				}
			}
		}

		// If there are additions and the campaign is already approved..
		if len(additions) > 0 && cmp.Approved > 0 {
			go emailList(s, cmp.Id, additions)
		}

		if upd.Perks != nil && cmp.Perks != nil {
			// Only update if the campaign already has perks..
			if cmp.Perks.IsCoupon() && upd.Perks.IsCoupon() {
				// If the saved perk is a coupon.. lets add more!
				existingCoupons := make(map[string]int)
				for _, cp := range cmp.Perks.Codes {
					existingCoupons[cp] += 1
				}

				// Get all the coupons saved in the deals
				var inUse bool
				for _, d := range cmp.Deals {
					if d.Perk != nil && d.Perk.Code != "" {
						existingCoupons[d.Perk.Code] += 1
						inUse = true
					}
				}

				newCouponMap := make(map[string]int)
				for _, newCouponCode := range upd.Perks.Codes {
					newCouponMap[newCouponCode] += 1
				}

				var (
					filteredList []string
					modified     bool
				)
				for cp, newVal := range newCouponMap {
					oldVal, _ := existingCoupons[cp]
					if oldVal != newVal {
						modified = true
						if newVal > oldVal {
							for i := 0; i < newVal-oldVal; i++ {
								filteredList = append(filteredList, cp)
							}
						}
					}
				}

				// If modified is true that means an existing coupon was either increased
				// in count or decreased. However, that (the above forloop and it's var modified
				// logic) wouldn't account for a coupon completely disappearing from the
				// newCouponMap
				if modified || len(newCouponMap) != len(existingCoupons) {
					dealsToAdd := len(filteredList)
					if dealsToAdd > 0 {
						// There are new coupons being added!
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
					} else {
						// Coupons are beign taken away since filtered returned nothing!
						if inUse {
							misc.WriteJSON(c, 400, misc.StatusErr("Cannot delete coupon codes that are in use by influencers"))
							return
						}

						cmp.Perks.Codes = upd.Perks.Codes
						cmp.Perks.Count = len(upd.Perks.Codes)

						if err = s.db.Update(func(tx *bolt.Tx) (err error) {
							resetDeals(&cmp, cmp.Perks.Count, s, tx)
							return saveCampaign(tx, &cmp, s)
						}); err != nil {
							misc.AbortWithErr(c, 500, err)
							return
						}
					}
				}
			} else if !cmp.Perks.IsCoupon() && !upd.Perks.IsCoupon() {
				// If the saved perk is a physical product.. lets add more!
				var bookedPerks int
				perksInUse := cmp.Perks.Count

				// Get all the products saved in the deals
				for _, d := range cmp.Deals {
					if d.Perk != nil {
						bookedPerks += d.Perk.Count
					}
				}
				perksInUse += bookedPerks

				if perksInUse > upd.Perks.Count {
					// Lets only allow decreasing perks IF the campaign has no deals yet
					if bookedPerks == 0 && (upd.Perks.Count > 0 && cmp.Perks.Count > upd.Perks.Count) {
						// Delete the extra deals we made
						var deleted int
						toDelete := cmp.Perks.Count - upd.Perks.Count
						for dealKey, deal := range cmp.Deals {
							if deleted >= toDelete {
								break
							}

							if deal.IsAvailable() {
								delete(cmp.Deals, dealKey)
								deleted += 1
							}
						}

						// Lower the perk count AFTER deleting the deals
						cmp.Perks.Count = upd.Perks.Count
					} else {
						misc.WriteJSON(c, 400, misc.StatusErr("Perk count can only be increased"))
						return
					}
				} else {
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
				store, _ := budget.GetCampaignStore(tx, s.Cfg, cmp.Id, cmp.AdvertiserId)
				if store == nil {
					// This means the campaign has no store.. cmp was craeted with a status of
					// off so a budget key was NOT created.. so we have to create it now!
					// NOTE: Create budget key requires cmp.Id be set
					if err = budget.Create(tx, s.Cfg, &cmp, ag.IsIO, adv.Customer); err != nil {
						s.Alert("Error initializing budget key for "+adv.Name, err)
						misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
						return
					}

					addDealsToCampaign(&cmp, s, tx, cmp.Budget)
				} else {
					// This campaign does have a store.. so it was active sometime this month.
					// Lets just give it the spendable we must have taken from it when it turned
					// off
					err = budget.ReplenishSpendable(tx, s.Cfg, &cmp, ag.IsIO, adv.Customer)
					if err != nil {
						misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
				spendable, err = budget.ClearSpendable(tx, s.Cfg, &cmp)
				if err != nil {
					misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
					return
				}

				// If we cleared out some spendable.. lets increment the balance with it
				if spendable > 0 {
					if err = budget.IncrBalance(cmp.AdvertiserId, spendable, tx, s.Cfg); err != nil {
						misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		if turnedOff {
			// Lets disactivate all currently ASSIGNED deals
			// and email the influencer to alert them if the campaign was turned off
			go func() {
				// Wait 15 mins before emailing
				emailStatusUpdate(s, cmp.Id)
			}()
		}

		misc.WriteJSON(c, 200, misc.StatusOK(cmp.Id))
	}
}

func getLatestGeo(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, inf.GetLatestGeo())
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
				if (cmp.Approved == 0 && cmp.Status) || (cmp.Perks != nil && cmp.Perks.PendingCount > 0) {
					// Hide deals
					cmp.Deals = nil
					campaigns = append(campaigns, &cmp)
				}
				return
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}
		misc.WriteJSON(c, 200, campaigns)
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
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid campaign ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(cId))
			return nil
		})

		if err = json.Unmarshal(b, &cmp); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		user := s.auth.GetUser(cmp.AdvertiserId)
		if user == nil || user.Advertiser == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
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
			email := templates.NotifyPerkEmail.Render(map[string]interface{}{"Perk": cmp.Perks.Name, "Name": user.Advertiser.Name})
			emailAdvertiser(s, user, email, "Your shipment has been received!")
		}

		// Bail early if this JUST an acceptance for a perk increase!
		if cmp.Approved > 0 {
			misc.WriteJSON(c, 200, misc.StatusOK(cmp.Id))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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

		misc.WriteJSON(c, 200, misc.StatusOK(cmp.Id))
	}
}

func uploadImage(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var upd UploadImage
		if err := json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		id := c.Param("id")
		if id == "" {
			misc.WriteJSON(c, 400, misc.StatusErr("Invalid ID"))
			return
		}

		bucket := c.Param("bucket")
		filename, err := saveImageToDisk(s.Cfg.ImagesDir+bucket+"/"+id, upd.Data, id, "", 750, 389)
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
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
				misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling campaign"))
				return
			}

			imageURL = getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false)
			cmp.ImageURL = imageURL

			// Save the Campaign
			if err = s.db.Update(func(tx *bolt.Tx) (err error) {
				return saveCampaign(tx, &cmp, s)
			}); err != nil {
				misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
				return
			}
		}
		misc.WriteJSON(c, 200, UploadImage{ImageURL: imageURL})
	}
}
