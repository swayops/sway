package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/missionMeteora/mandrill"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/hellosign"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/swipe"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

///////// Talent Agencies ///////////
func putTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "talentAgency")
	}
}

func getTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ag *auth.TalentAgency
		s.db.View(func(tx *bolt.Tx) error {
			ag = s.auth.GetTalentAgencyTx(tx, c.Param("id"))
			return nil
		})
		if ag == nil {
			misc.AbortWithErr(c, 400, auth.ErrInvalidAgencyID)
			return
		}
		c.JSON(200, ag)
	}
}

func getAllTalentAgencies(s *Server) gin.HandlerFunc {
	type userWithCounts struct {
		*auth.User
		SubCount int64 `json:"subCount"`
	}
	return func(c *gin.Context) {
		var (
			all []*userWithCounts
		)

		s.db.View(func(tx *bolt.Tx) error {
			s.auth.GetUsersByTypeTx(tx, auth.TalentAgencyScope, func(u *auth.User) error {
				if u.TalentAgency != nil {
					all = append(all, &userWithCounts{u.Trim(), s.auth.Influencers.GetCount(u.ID)})
				}
				return nil
			})
			return nil
		})
		c.JSON(200, all)
	}
}

///////// Ad Agencies /////////
func putAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "adAgency")
	}
}

func getAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ag *auth.AdAgency
		s.db.View(func(tx *bolt.Tx) error {
			ag = s.auth.GetAdAgencyTx(tx, c.Param("id"))
			return nil
		})
		if ag == nil {
			misc.AbortWithErr(c, 400, auth.ErrInvalidAgencyID)
			return
		}
		c.JSON(200, ag)
	}
}

func getAllAdAgencies(s *Server) gin.HandlerFunc {
	type userWithCounts struct {
		*auth.User
		SubCount int `json:"subCount"`
	}
	return func(c *gin.Context) {
		var (
			all    []*userWithCounts
			counts map[string]int
			uids   []string
		)

		s.db.View(func(tx *bolt.Tx) error {
			s.auth.GetUsersByTypeTx(tx, auth.AdAgencyScope, func(u *auth.User) error {
				if u.AdAgency != nil { // should always be true, but just in case
					all = append(all, &userWithCounts{u.Trim(), 0})
					uids = append(uids, u.ID)
				}
				return nil
			})
			counts = s.auth.GetChildCountsTx(tx, uids...)
			return nil
		})

		for _, u := range all {
			u.SubCount = counts[u.ID]
		}
		c.JSON(200, all)
	}
}

func putAdmin(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "admin")

	}
}

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "advertiser")
	}
}

func getAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		adv := s.auth.GetAdvertiser(c.Param("id"))
		if adv == nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, adv)
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

func getAdvertisersByAgency(s *Server) gin.HandlerFunc {
	type advWithCounts struct {
		*auth.User
		NumCampaigns int `json:"numCmps"`
	}
	return func(c *gin.Context) {
		var (
			targetAgency = c.Param("id")
			advertisers  []*advWithCounts
			counts       = map[string]int{}
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			if u := s.auth.GetUserTx(tx, targetAgency); u == nil || u.Type() != auth.AdAgencyScope {
				return auth.ErrInvalidUserID
			}
			s.auth.GetUsersByTypeTx(tx, auth.AdvertiserScope, func(u *auth.User) error {
				if u.Advertiser != nil && u.ParentID == targetAgency {
					advertisers = append(advertisers, &advWithCounts{u.Trim(), 0})
					counts[u.ID] = 0
				}
				return nil
			})
			return tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp struct {
					AdvertiserId string `json:"advertiserId"`
				}
				if json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if _, ok := counts[cmp.AdvertiserId]; ok {
					counts[cmp.AdvertiserId]++
				}
				return
			})
		}); err != nil {
			misc.AbortWithErr(c, 404, err)
			return
		}
		for _, adv := range advertisers {
			adv.NumCampaigns = counts[adv.ID]
		}
		c.JSON(200, advertisers)
	}
}

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

		if cmp.Budget < 150 {
			c.JSON(400, misc.StatusErr("Please provide a valid budget"))
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

func getCampaignStore(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, s.Campaigns.GetStore())
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

///////// Influencers /////////
var (
	ErrBadGender = errors.New("Please provide a gender ('m' or 'f')")
	ErrNoAgency  = errors.New("Please provide an agency id")
	ErrNoGeo     = errors.New("Please provide a geo")
	ErrNoName    = errors.New("Please provide a valid name")
	ErrBadCat    = errors.New("Please provide a valid category")
	ErrPlatform  = errors.New("Platform not found!")
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
)

type InfluencerUpdate struct {
	Name        *string         `json:"name,omitempty"` // Required to send
	Phone       *string         `json:"phone,omitempty"`
	InstagramId string          `json:"instagram,omitempty"`         // Required to send
	FbId        string          `json:"facebook,omitempty"`          // Required to send
	TwitterId   string          `json:"twitter,omitempty"`           // Required to send
	YouTubeId   string          `json:"youtube,omitempty"`           // Required to send
	DealPing    *bool           `json:"dealPing" binding:"required"` // Required to send
	Address     lob.AddressLoad `json:"address,omitempty"`           // Required to send

	InviteCode string `json:"inviteCode,omitempty"` // Optional

	// User methods
	OldPass string `json:"oldPass"` // Optional
	Pass    string `json:"pass"`    // Optional
	Pass2   string `json:"pass2"`   // Optional

	ImageURL      string `json:"imageUrl,omitempty"`      // Optional
	CoverImageURL string `json:"coverImageUrl,omitempty"` // Optional
}

func putInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("id"))
		if !ok {
			c.JSON(500, misc.StatusErr("Please provide a valid influencer ID"))
			return
		}

		var (
			upd InfluencerUpdate
			err error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		// Update platforms
		if upd.InstagramId != "" {
			if inf.Instagram == nil || (inf.Instagram != nil && upd.InstagramId != inf.Instagram.UserName) {
				// Make sure that the instagram id has actually been updated
				keywords := getScrapKeywords(s, inf.EmailAddress, upd.InstagramId)
				err = inf.NewInsta(upd.InstagramId, keywords, s.Cfg)
				if err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		} else {
			// If the ID is sent as empty, they'll be emptied out
			inf.Instagram = nil
		}

		if upd.FbId != "" {
			if inf.Facebook == nil || (inf.Facebook != nil && upd.FbId != inf.Facebook.Id) {
				// Make sure that the id has actually been updated
				err = inf.NewFb(upd.FbId, s.Cfg)
				if err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		} else {
			// If the ID is sent as empty, they'll be emptied out
			inf.Facebook = nil
		}

		if upd.TwitterId != "" {
			if inf.Twitter == nil || (inf.Twitter != nil && upd.TwitterId != inf.Twitter.Id) {
				// Make sure that the id has actually been updated
				err = inf.NewTwitter(upd.TwitterId, s.Cfg)
				if err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		} else {
			inf.Twitter = nil
		}

		if upd.YouTubeId != "" {
			if inf.YouTube == nil || (inf.YouTube != nil && upd.YouTubeId != inf.YouTube.UserName) {
				keywords := getScrapKeywords(s, inf.EmailAddress, upd.YouTubeId)
				// Make sure that the id has actually been updated
				err = inf.NewYouTube(upd.YouTubeId, keywords, s.Cfg)
				if err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		} else {
			// If the ID is sent as empty, they'll be emptied out
			inf.YouTube = nil
		}

		// Update Invite Code
		if upd.InviteCode != "" {
			agencyId := common.GetIDFromInvite(upd.InviteCode)
			if agencyId == "" {
				agencyId = auth.SwayOpsTalentAgencyID
			}
			inf.AgencyId = agencyId
		}

		// Update DealPing
		if upd.DealPing != nil {
			// Set to a pointer so we don't default to
			// false incase front end doesnt send the value
			inf.DealPing = *upd.DealPing
		}

		// Update Address
		if upd.Address.AddressOne != "" {
			cleanAddr, err := lob.VerifyAddress(&upd.Address, s.Cfg)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			if !geo.IsValidGeo(&geo.GeoRecord{State: cleanAddr.State, Country: cleanAddr.Country}) {
				c.JSON(400, misc.StatusErr("Address does not convert to a valid geo!"))
				return
			}

			inf.Address = cleanAddr
		}

		// Update User properties
		var user *auth.User
		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			user = s.auth.GetUserTx(tx, inf.Id)
			if user == nil {
				return auth.ErrInvalidID
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if upd.Name != nil {
			name := strings.TrimSpace(*upd.Name)
			if len(strings.Split(name, " ")) < 2 {
				c.JSON(400, misc.StatusErr(ErrNoName.Error()))
				return
			}

			user.Name = name
		}

		if upd.Phone != nil {
			user.Phone = strings.TrimSpace(*upd.Phone)
		}

		user.ImageURL, err = getUserImage(s, upd.ImageURL, "", 168, 168, user)
		if err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		user.CoverImageURL, err = getUserImage(s, upd.CoverImageURL, "-cover", 300, 150, user)
		if err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		user.ParentID = inf.AgencyId

		if err := s.db.Update(func(tx *bolt.Tx) error {
			changed, err := savePassword(s, tx, upd.OldPass, upd.Pass, upd.Pass2, user)
			if err != nil {
				return err
			}

			if changed {
				ouser := s.auth.GetUserTx(tx, user.ID) // always reload after changing the password
				user = ouser.Update(user)
			}

			return saveInfluencerWithUser(s, tx, inf, user)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

type AuditSet struct {
	Categories []string `json:"categories,omitempty"`
	Gender     string   `json:"gender,omitempty"`
}

func setAudit(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Please provide a valid influencer ID"))
			return
		}

		var (
			upd AuditSet
			err error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		var filteredCats []string
		for _, cat := range upd.Categories {
			if _, ok := common.CATEGORIES[cat]; !ok {
				c.JSON(400, misc.StatusErr(ErrBadCat.Error()))
				return
			}
			filteredCats = append(filteredCats, cat)
		}

		inf.Categories = filteredCats

		switch upd.Gender {
		case "mf", "fm", "unicorn":
			inf.Male, inf.Female = true, true
		case "m":
			inf.Male, inf.Female = true, false
		case "f":
			inf.Male, inf.Female = false, true
		}

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

func getInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("id"))
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, inf)
	}
}

type Bio struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name,omitempty"`
	Networks []string `json:"networks,omitempty"`

	Deals       int32 `json:"deals,omitempty"` // # of deals completed
	Followers   int64 `json:"followers,omitempty"`
	Engagements int64 `json:"engagements,omitempty"`

	CompletedDeals []*BioDeal `json:"completedDeals,omitempty"`
}

type BioDeal struct {
	ID          string `json:"id,omitempty"`
	CampaignID  string `json:"campaignId,omitempty"`
	Name        string `json:"cmpName,omitempty"`
	Engagements int64  `json:"engagements,omitempty"`
	Image       string `json:"image,omitempty"`
}

func getBio(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		var (
			eng      int64
			bioDeals []*BioDeal
		)
		for _, deal := range inf.CompletedDeals {
			total := deal.TotalStats()
			dealEng := int64(total.Likes + total.Comments + total.Shares + total.GetClicks())

			eng += dealEng

			d := &BioDeal{
				ID:          deal.Id,
				CampaignID:  deal.CampaignId,
				Engagements: dealEng,
				Image:       deal.CampaignImage,
				Name:        deal.CampaignName,
			}
			bioDeals = append(bioDeals, d)
		}

		bio := &Bio{
			ID:             inf.Id,
			Name:           inf.Name,
			Networks:       inf.GetNetworks(),
			Deals:          int32(len(inf.CompletedDeals)),
			Followers:      inf.GetFollowers(),
			Engagements:    eng,
			CompletedDeals: bioDeals,
		}
		c.JSON(200, bio)
	}
}

func getCompletedDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		dealId := c.Param("dealId")
		if dealId == "" {
			c.JSON(500, misc.StatusErr("invalid deal id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		var d *common.Deal
		for _, deal := range inf.CompletedDeals {
			if deal.Id == dealId {
				d = deal
				break
			}
		}

		if d == nil {
			c.JSON(500, misc.StatusErr("deal not found"))
			return
		}

		c.JSON(200, d)
	}
}

func getInfluencersByCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var influencers []influencer.Influencer
		targetCat := c.Param("category")

		for _, inf := range s.auth.Influencers.GetAll() {
			for _, infCat := range inf.Categories {
				if infCat == targetCat {
					inf.Clean()
					influencers = append(influencers, inf)
				}
			}
		}
		c.JSON(200, influencers)
	}
}

func getInfluencersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var influencers []influencer.Influencer
		targetAg := c.Param("id")
		for _, inf := range s.auth.Influencers.GetAll() {
			if inf.AgencyId == targetAg {
				inf.Followers = inf.GetFollowers()
				inf.Clean()
				if len(inf.CompletedDeals) != 0 {
					st := reporting.GetInfluencerBreakdown(inf, s.Cfg, -1, inf.Rep, inf.CurrentRep, "", inf.AgencyId)
					total := st["total"]
					if total != nil {
						inf.AgencySpend = total.AgencySpent
						inf.InfluencerSpend = total.Spent
					}
				}
				influencers = append(influencers, inf)
			}
		}
		c.JSON(200, influencers)
	}
}

func setBan(s *Server) gin.HandlerFunc {
	// Sets the banned value for the influencer id
	return func(c *gin.Context) {
		ban, err := strconv.ParseBool(c.Params.ByName("state"))
		if err != nil {
			c.JSON(400, misc.StatusErr("Please submit a valid ban state"))
			return
		}

		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.Banned = ban

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Influencer has been banned!", fmt.Sprintf("Influencer %s has been banned", infId))

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setStrike(s *Server) gin.HandlerFunc {
	// Sets the banned value for the influencer id
	return func(c *gin.Context) {
		reasons := c.Params.ByName("reasons")
		if reasons == "" {
			c.JSON(400, misc.StatusErr("Please submit a valid reason"))
			return
		}

		var (
			infId      = c.Param("influencerId")
			campaignId = c.Param("campaignId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if campaignId == "" {
			c.JSON(500, misc.StatusErr("Invalid campaign ID"))
			return
		}

		// Add a strike
		strike := &influencer.Strike{
			CampaignID: campaignId,
			Reasons:    reasons,
			TS:         time.Now().Unix(),
		}

		// Make sure it's not there already!
		for _, st := range inf.Strikes {
			if st.CampaignID == strike.CampaignID {
				c.JSON(500, misc.StatusErr("Strike has already been recorded!"))
				return
			}
		}

		inf.Strikes = append(inf.Strikes, strike)

		// Allow the deal by skipping fraud
		for _, d := range inf.ActiveDeals {
			if d.CampaignId == campaignId {
				d.SkipFraud = true
			}
		}

		if err := saveAllActiveDeals(s, inf); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Strike given!", fmt.Sprintf("Influencer %s has been given a strike (and the post has been allowed) for campaign %s", infId, campaignId))

		c.JSON(200, misc.StatusOK(infId))
	}
}

func addKeyword(s *Server) gin.HandlerFunc {
	// Manually add kw
	return func(c *gin.Context) {
		kw := c.Param("kw")
		if kw == "" {
			c.JSON(400, misc.StatusErr("Please submit a valid keyword"))
			return
		}

		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.Keywords = append(inf.Keywords, kw)

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setSignature(s *Server) gin.HandlerFunc {
	// Manually set sig id
	return func(c *gin.Context) {
		sigId := c.Param("sigId")
		if sigId == "" {
			c.JSON(400, misc.StatusErr("Please submit a valid sigId"))
			return
		}

		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.SignatureId = sigId
		inf.RequestedCheck = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func addDealCount(s *Server) gin.HandlerFunc {
	// Manually add a certain number of deals
	return func(c *gin.Context) {
		count, err := strconv.Atoi(c.Param("count"))
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		var (
			cmp common.Campaign
			b   []byte
		)

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(c.Param("campaignId")))
			return nil
		})

		if err = json.Unmarshal(b, &cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		if cmp.Perks != nil {
			c.JSON(400, misc.StatusErr("Cannot add deals to perk campaign"))
			return
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			addDeals(&cmp, count, s, tx)
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

var (
	InvalidPostURL = errors.New("Invalid post URL!")
)

type Bonus struct {
	CampaignID   string `json:"cmpID,omitempty"`
	InfluencerID string `json:"infID,omitempty"`
	PostURL      string `json:"url,omitempty"`
}

func addBonus(s *Server) gin.HandlerFunc {
	// Adds bonus value to an existing completed deal
	return func(c *gin.Context) {
		var (
			bonus Bonus
			err   error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&bonus); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		if bonus.InfluencerID == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		if bonus.CampaignID == "" {
			c.JSON(500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(bonus.InfluencerID)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		// Force update saves all new posts and updates to recent data
		err = inf.ForceUpdate(s.Cfg)
		if err != nil {
			c.JSON(500, misc.StatusErr("internal error with influencer update"))
			return
		}

		parsed, err := url.Parse(bonus.PostURL)
		if err != nil {
			c.JSON(500, misc.StatusErr("invalid post URL"))
			return
		}

		bonus.PostURL = parsed.Host + parsed.Path
		if bonus.PostURL == "" {
			c.JSON(500, misc.StatusErr("invalid post URL"))
			return
		}

		var foundDeal *common.Deal
		for _, d := range inf.CompletedDeals {
			if d.CampaignId == bonus.CampaignID {
				foundDeal = d
			}
		}

		if foundDeal == nil {
			c.JSON(500, misc.StatusErr("deal not found"))
			return
		}

		var foundURL bool
		if inf.Twitter != nil {
			for _, tw := range inf.Twitter.LatestTweets {
				if strings.Contains(tw.PostURL, bonus.PostURL) {
					foundDeal.AddBonus(tw, nil, nil, nil)
					foundURL = true
					break
				}
			}
		}

		if inf.Facebook != nil {
			for _, fb := range inf.Facebook.LatestPosts {
				if strings.Contains(fb.PostURL, bonus.PostURL) {
					foundDeal.AddBonus(nil, fb, nil, nil)
					foundURL = true
					break
				}
			}
		}

		if inf.Instagram != nil {
			for _, in := range inf.Instagram.LatestPosts {
				if strings.Contains(in.PostURL, bonus.PostURL) {
					foundDeal.AddBonus(nil, nil, in, nil)
					foundURL = true
					break
				}
			}
		}

		if inf.YouTube != nil {
			for _, yt := range inf.YouTube.LatestPosts {
				if strings.Contains(yt.PostURL, bonus.PostURL) {
					foundDeal.AddBonus(nil, nil, nil, yt)
					foundURL = true
					break
				}
			}
		}

		if !foundURL {
			c.JSON(500, misc.StatusErr("invalid post URL"))
			return
		}

		if err := saveAllCompletedDeals(s, inf); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(bonus.InfluencerID))
	}
}

func setFraud(s *Server) gin.HandlerFunc {
	// Sets the fraud check value for a deal
	return func(c *gin.Context) {
		fraud, err := strconv.ParseBool(c.Params.ByName("state"))
		if err != nil {
			c.JSON(400, misc.StatusErr("Please submit a valid fraud state"))
			return
		}

		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		cid := c.Param("campaignId")
		if cid == "" {
			c.JSON(500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		for _, d := range inf.ActiveDeals {
			if d.CampaignId == cid {
				d.SkipFraud = fraud
			}
		}

		if err := saveAllActiveDeals(s, inf); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Deal post allowed!", fmt.Sprintf("Deal for campaign %s and influencer %s has been allowed", cid, infId))

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setAgency(s *Server) gin.HandlerFunc {
	// Helper handler for setting the agency for the influencer id
	return func(c *gin.Context) {
		var (
			infId = c.Param("influencerId")
			agId  = c.Param("agencyId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		talentAgency := s.auth.GetTalentAgency(agId)
		if talentAgency == nil {
			c.JSON(500, misc.StatusErr(fmt.Sprintf("Could not find talent agency %s", inf.AgencyId)))
			return
		}

		inf.AgencyId = agId

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

type IncompleteInfluencer struct {
	influencer.Influencer
	FacebookURL  string `json:"facebookUrl,omitempty"`
	InstagramURL string `json:"instagramUrl,omitempty"`
	TwitterURL   string `json:"twitterUrl,omitempty"`
	YouTubeURL   string `json:"youtubeUrl,omitempty"`
}

func getIncompleteInfluencers(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			influencers []*IncompleteInfluencer
			incPosts, _ = strconv.ParseBool(c.Query("incPosts"))
		)
		for _, inf := range s.auth.Influencers.GetAll() {
			if inf.IsBanned() {
				continue
			}

			if (!inf.Male && !inf.Female) || len(inf.Categories) == 0 {
				var (
					incInf IncompleteInfluencer
					found  bool
				)

				if inf.Twitter != nil {
					incInf.TwitterURL, found = inf.Twitter.GetProfileURL(), true
					if !incPosts {
						inf.Twitter = nil
					}
				}

				if inf.Facebook != nil {
					incInf.FacebookURL, found = inf.Facebook.GetProfileURL(), true
					if !incPosts {
						inf.Facebook = nil
					}
				}

				if inf.Instagram != nil {
					incInf.InstagramURL, found = inf.Instagram.GetProfileURL(), true
					if !incPosts {
						inf.Instagram = nil
					}
				}

				if inf.YouTube != nil {
					incInf.YouTubeURL, found = inf.YouTube.GetProfileURL(), true
					if !incPosts {
						inf.YouTube = nil
					}
				}

				if found {
					incInf.Influencer = inf
					influencers = append(influencers, &incInf)
				}
			}
		}
		c.JSON(200, influencers)
	}
}

type InfCategory struct {
	Category    string `json:"cat,omitempty"`
	Influencers int64  `json:"infs,omitempty"`
	Reach       int64  `json:"reach,omitempty"`
}

func findCat(haystack []*InfCategory, cat string) *InfCategory {
	for _, i := range haystack {
		if i.Category == cat {
			return i
		}
	}
	return nil
}

func getCategories(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Returns a map with key as the category
		// and value as reach
		out := make([]*InfCategory, 0, len(common.CATEGORIES))
		for k, _ := range common.CATEGORIES {
			out = append(out, &InfCategory{Category: k})
		}

		for _, inf := range s.auth.Influencers.GetAll() {
			for _, cat := range inf.Categories {
				if val := findCat(out, cat); val != nil {
					val.Influencers += 1
					val.Reach += inf.GetFollowers()
				}
			}
		}

		c.JSON(200, out)
	}
}

///////// Deals /////////
func getDealsForCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, misc.StatusErr(fmt.Sprintf("Failed for campaign")))
			return
		}

		c.JSON(200, getDealsForCmp(s, cmp, false))
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

type Match struct {
	Id       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Insta    string `json:"insta,omitempty"`
	Facebook string `json:"facebook,omitempty"`
	YouTube  string `json:"youtube,omitempty"`
	Twitter  string `json:"twitter,omitempty"`
}

func getMatchesForKeyword(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		kw := c.Param("kw")
		matches := []*Match{}
		for _, inf := range s.auth.Influencers.GetAll() {
			if common.IsInList(inf.Keywords, kw) {
				inf = *inf.Clean()
				matches = append(matches, &Match{
					Id:       inf.Id,
					Type:     "influencer",
					Insta:    inf.InstaUsername,
					Facebook: inf.FbUsername,
					YouTube:  inf.YTUsername,
					Twitter:  inf.TwitterUsername,
				})
			}
		}

		scraps, _ := getAllScraps(s)
		for _, sc := range scraps {
			if common.IsInList(sc.Keywords, kw) {
				m := &Match{
					Id:   sc.Id,
					Type: "scrap",
				}

				if sc.Instagram {
					m.Insta = sc.Name
				}
				if sc.Twitter {
					m.Twitter = sc.Name
				}

				if sc.Facebook {
					m.Facebook = sc.Name
				}

				if sc.Twitter {
					m.Twitter = sc.Name
				}

				matches = append(matches, m)

			}
		}

		c.JSON(200, matches)
	}
}

func getDealsForInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			lat, _  = strconv.ParseFloat(c.Param("lat"), 64)
			long, _ = strconv.ParseFloat(c.Param("long"), 64)
			infId   = c.Param("influencerId")
		)

		if len(infId) == 0 {
			c.JSON(500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		deals := inf.GetAvailableDeals(s.Campaigns, s.budgetDb, "", "",
			geo.GetGeoFromCoords(lat, long, int32(time.Now().Unix())), false, s.Cfg)
		c.JSON(200, deals)
	}
}

func getDeal(s *Server) gin.HandlerFunc {
	// Gets assigned deal using GetAvailableDeals func so we can make sure
	// the campaign still wants this influencer!
	return func(c *gin.Context) {
		var (
			campaignId = c.Param("campaignId")
			dealId     = c.Param("dealId")
			infId      = c.Param("influencerId")
		)

		if len(infId) == 0 {
			c.JSON(500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		if len(dealId) == 0 {
			c.JSON(500, misc.StatusErr("Deal ID undefined"))
			return
		}

		if len(campaignId) == 0 {
			c.JSON(500, misc.StatusErr("Campaign ID undefined"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		deals := inf.GetAvailableDeals(s.Campaigns, s.budgetDb, campaignId, dealId, nil, true, s.Cfg)
		if len(deals) != 1 {
			c.JSON(500, misc.StatusErr("Deal no longer available"))
			return
		}
		c.JSON(200, deals[0])
	}
}

func assignDeal(s *Server) gin.HandlerFunc {
	// Influencer accepting deal
	// Must pass in influencer ID and deal ID
	return func(c *gin.Context) {
		var (
			infId         = c.Param("influencerId")
			dealId        = c.Param("dealId")
			campaignId    = c.Param("campaignId")
			mediaPlatform = c.Param("platform")
		)

		if _, ok := platform.ALL_PLATFORMS[mediaPlatform]; !ok {
			c.JSON(500, misc.StatusErr("This platform was not found"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		// Lets quickly make sure that this deal is still available
		// via our GetAvailableDeals func
		var (
			found, dbg bool
		)

		foundDeal := &common.Deal{}
		if c.Query("dbg") == "1" {
			// In debug state.. all deals are recovered and random is assigned from the campaign given
			dealId = ""
			dbg = true
		}

		currentDeals := inf.GetAvailableDeals(s.Campaigns, s.budgetDb, campaignId, dealId, nil, false, s.Cfg)
		for _, deal := range currentDeals {
			if deal.Spendable > 0 && deal.CampaignId == campaignId && deal.Assigned == 0 && deal.InfluencerId == "" {
				if dbg || deal.Id == dealId {
					found = true
					foundDeal = deal
				}
			}
		}

		if !found {
			c.JSON(500, misc.StatusErr("Unforunately, the requested deal is no longer available!"))
			return
		}

		// Assign the deal & Save the Campaign
		// DEALS are located in the INFLUENCER struct AND the CAMPAIGN struct
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var cmp *common.Campaign

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(campaignId)), &cmp)
			if err != nil {
				return err
			}

			if !cmp.IsValid() {
				return errors.New("Campaign is no longer active")
			}

			// Check if any perks are left to give this dude
			if cmp.Perks != nil {
				if cmp.Perks.Count == 0 {
					return errors.New("Deal is no longer available!")
				}

				if inf.Address == nil {
					return errors.New("Please enter a valid mailing address in your profile before accepting this deal")
				}

				// Now that we know there is a deal for this dude..
				// and they have an address.. schedule a perk order!

				cmp.Perks.Count -= 1
				foundDeal.Perk = &common.Perk{
					Name:         cmp.Perks.Name,
					Instructions: cmp.Perks.Instructions,
					Category:     cmp.Perks.GetType(),
					Count:        1,
					InfId:        inf.Id,
					InfName:      inf.Name,
					Address:      inf.Address,
					Status:       false,
				}

				if cmp.Perks.Count == 0 {
					// Lets email the advertiser letting them know there are no more
					// perks available!

					user := s.auth.GetUser(cmp.AdvertiserId)
					if user == nil || user.Advertiser == nil {
						c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
						return
					}

					if s.Cfg.ReplyMailClient() != nil {
						email := templates.NotifyEmptyPerkEmail.Render(map[string]interface{}{"ID": user.ID, "Campaign": cmp.Name, "Perk": cmp.Perks.Name, "Name": user.Advertiser.Name})
						resp, err := s.Cfg.ReplyMailClient().SendMessage(email, fmt.Sprintf("You have no remaining perks for the campaign "+cmp.Name), user.Email, user.Name,
							[]string{""})
						if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
							s.Alert("Failed to mail advertiser regarding perks running out", err)
						} else {
							if err := s.Cfg.Loggers.Log("email", map[string]interface{}{
								"tag": "no more perks",
								"id":  user.ID,
							}); err != nil {
								log.Println("Failed to log out of perks notify email!", user.ID)
							}
						}
					}
				}

				// If it's a coupon code.. we do not need admin approval
				// so lets set the status to true
				if cmp.Perks.IsCoupon() {
					if len(cmp.Perks.Codes) == 0 {
						return errors.New("Deal is no longer available!")
					}

					foundDeal.Perk.Status = true
					// Give it last element of the slice
					idx := len(cmp.Perks.Codes) - 1
					foundDeal.Perk.Code = cmp.Perks.Codes[idx]

					// Lets also delete the coupon code
					cmp.Perks.Codes = cmp.Perks.Codes[:idx]
				} else {
					s.Notify("Perk requested!", fmt.Sprintf("%s just requested a perk (%s) to be mailed to them! Please check admin dash.", inf.Name, cmp.Perks.Name))
				}
			}

			foundDeal.InfluencerId = infId
			foundDeal.InfluencerName = inf.Name
			foundDeal.Assigned = int32(time.Now().Unix())

			if len(foundDeal.Platforms) == 0 {
				return errors.New("Unforunately, the requested deal is no longer available!")
			}

			cmp.Deals[foundDeal.Id] = foundDeal

			// Append to the influencer's active deals
			inf.ActiveDeals = append(inf.ActiveDeals, foundDeal)

			// Lets add to timeline
			cmp.AddToTimeline(common.DEAL_ACCEPTED, true, s.Cfg)

			// Save the Influencer
			if err = saveInfluencer(s, tx, inf); err != nil {
				return
			}

			// Save the campaign
			if err = saveCampaign(tx, cmp, s); err != nil {
				return
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, foundDeal)
	}
}

func getDealsAssignedToInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		var deals []*common.Deal
		for _, d := range inf.ActiveDeals {
			deals = append(deals, d)
		}

		c.JSON(200, deals)
	}
}

func unassignDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		dealId := c.Param("dealId")
		influencerId := c.Param("influencerId")
		campaignId := c.Param("campaignId")

		if err := clearDeal(s, dealId, influencerId, campaignId, false); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Lets email the influencer telling them the deal is OVAH!
		inf, ok := s.auth.Influencers.Get(influencerId)
		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)

		if ok && cmp != nil {
			if err := inf.DealUpdate(cmp, s.Cfg); err != nil {
				s.Alert("Failed to give influencer a deal update: "+inf.Id, err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
		}

		c.JSON(200, misc.StatusOK(dealId))
	}
}

func getDealsCompletedByInfluencer(s *Server) gin.HandlerFunc {
	// Get all deals completed by the influencer in the last X hours
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, inf.CompletedDeals)
	}
}

// Budget
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

func getCampaignReport(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.Param("cid")
		if cid == "" {
			c.JSON(500, misc.StatusErr("Please pass in a valid campaign ID"))
			return
		}

		from := reporting.GetReportDate(c.Param("from"))
		to := reporting.GetReportDate(c.Param("to"))
		if from.IsZero() || to.IsZero() || to.Before(from) {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		if err := reporting.GenerateCampaignReport(c.Writer, s.db, cid, from, to, s.Cfg); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
		}
	}
}

type AdminStats struct {
	AdAgencies  int `json:"adAgencies"`  // Total # of Ad Agencies
	Advertisers int `json:"advertisers"` // Total # of Advertisers
	Campaigns   int `json:"cmps"`        // Total # of Campaigns

	PerksInbound   int `json:"perkInb"`     // Total # of Perks Inbound
	PerksStored    int `json:"perkStore"`   // Total # of Perks Stored
	PerksOutbound  int `json:"perkOut"`     // Total # of Perks Outbound
	PerksDelivered int `json:"perkDeliver"` // Total # of Perks Delivered

	DealsAccepted  int     `json:"dealAccepted"`   // Total # of Deals Accepted
	DealsCompleted int     `json:"dealCompleted"`  // Total # of Deals Completed
	CompletionRate float64 `json:"completionRate"` // Percentage of deals completed

	TalentAgencies    int     `json:"talentAgencies"`    // Total # of Talent Agencies
	InfPerTalent      int     `json:"infPerTalent"`      // # of Influencers per Talent Agency
	TotalAgencyPayout float64 `json:"totalAgencyPayout"` // Total $ paid out to Talent Agencies

	Influencers           int     `json:"influencers"`           // Total # of Influencers
	TotalInfluencerPayout float64 `json:"totalInfluencerPayout"` // Total $ paid out to Influencers
	Reach                 int64   `json:"reach"`                 // Total influencer reach
	Likes                 int32   `json:"likes"`                 // Total # of Likes generated by deal posts
	Comments              int32   `json:"comments"`              // Total # of Comments generated by deal posts
	Shares                int32   `json:"shares"`                // Total # of Shares generated by deal posts
	Views                 int32   `json:"views"`                 // Total # of Views generated by deal posts
	Clicks                int32   `json:"clicks"`                // Total # of Clicks generated by deal posts
}

func getAdminStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			perksInbound, perksStored, perksOutbound, perksDelivered, dealsAccept, dealsComplete int
			a                                                                                    *AdminStats
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if cmp.Approved == 0 {
					// This is a campaign who's perks we are waiting for! (inbound)
					if cmp.Perks != nil {
						perksInbound += cmp.Perks.Count
					}
				} else {
					// This is a campaign that's been approved (we have all their perks)
					if cmp.Perks != nil {
						perksStored += cmp.Perks.Count
					}
				}

				if !cmp.IsValid() {
					return nil
				}

				for _, d := range cmp.Deals {
					if d.Perk != nil && d.InfluencerId != "" {
						if !d.Perk.Status {
							// This deal has been picked up.. there's a perk attached
							// and the status is false (meaning it hasn't been mailed yet)
							perksOutbound += 1
						} else {
							// This deal is set to true meaning its been mailed!
							perksDelivered += 1
						}
					}

					if d.IsActive() {
						dealsAccept += 1
					}

					if d.IsComplete() {
						dealsComplete += 1
					}
				}
				return
			})

			talentAgencyCount := len(getTalentAgencies(s, tx))
			var (
				infCount                               int
				reach                                  int64
				likes, comments, shares, views, clicks int32
				totalInfluencer, totalAgency           float64
			)

			for _, inf := range s.auth.Influencers.GetAll() {
				reach += inf.GetFollowers()
				infCount += 1
				for _, d := range inf.CompletedDeals {
					stats := d.TotalStats()
					totalInfluencer += stats.Influencer
					totalAgency += stats.Agency
					likes += stats.Likes
					comments += stats.Comments
					shares += stats.Shares
					views += stats.Views
					clicks += stats.GetClicks()
				}
			}

			var completionRate float64
			if dealsComplete > 0 {
				completionRate = 100 * (float64(dealsComplete) / float64(dealsComplete+dealsAccept))
			}

			a = &AdminStats{
				AdAgencies:            len(getAdAgencies(s, tx)),
				Advertisers:           len(getAdvertisers(s, tx)),
				Campaigns:             s.Campaigns.Len(),
				PerksInbound:          perksInbound,
				PerksStored:           perksStored,
				PerksOutbound:         perksOutbound,
				PerksDelivered:        perksDelivered,
				DealsAccepted:         dealsAccept,
				DealsCompleted:        dealsComplete,
				CompletionRate:        completionRate,
				TalentAgencies:        talentAgencyCount,
				Influencers:           infCount,
				InfPerTalent:          int(float32(infCount) / float32(talentAgencyCount)),
				TotalAgencyPayout:     totalAgency,
				TotalInfluencerPayout: totalInfluencer,
				Reach:    reach,
				Likes:    likes,
				Comments: comments,
				Shares:   shares,
				Views:    views,
				Clicks:   clicks,
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, a)
	}
}

func getAdvertiserTimeline(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			targetAdv = c.Param("id")
		)

		cmpTimeline := make(map[string]*common.Timeline)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv && len(cmp.Timeline) > 0 {
					cmpTimeline[fmt.Sprintf("%s (%s)", cmp.Name, cmp.Id)] = cmp.Timeline[len(cmp.Timeline)-1]
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		common.SetLinkTitles(cmpTimeline)
		c.JSON(200, cmpTimeline)
	}
}

func getAdvertiserStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			start, _  = strconv.Atoi(c.Param("start"))
			end, _    = strconv.Atoi(c.Param("end"))
			targetAdv = c.Param("id")
			campaigns []*common.Campaign
			cmpStats  []map[string]*reporting.Totals
		)

		if start == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
					campaigns = append(campaigns, &cmp)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		for _, cmp := range campaigns {
			stats := reporting.GetCampaignBreakdown(cmp.Id, s.db, s.Cfg, start, end)
			cmpStats = append(cmpStats, stats)
		}

		c.JSON(200, reporting.Merge(cmpStats))
	}
}

func getCampaignStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		c.JSON(200, reporting.GetCampaignBreakdown(c.Param("cid"), s.db, s.Cfg, days, 0))
	}
}

func getInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		c.JSON(200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, "", ""))
	}
}

func getCampaignInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("infId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		c.JSON(200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, c.Param("cid"), ""))
	}
}

func getAgencyInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("infId"))
		if !ok {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		c.JSON(200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, "", c.Param("id")))
	}
}

// Billing

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

				if cmp.Budget == 0 {
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

type GreedyInfluencer struct {
	Id    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	SigID string `json:"sigId,omitempty"`

	Address   *lob.AddressLoad `json:"address,omitempty"`
	Followers int64            `json:"followers,omitempty"`
	// Post URLs for the complete deals since last check
	CompletedDeals []string `json:"completedDeals,omitempty"`
	PendingPayout  float64  `json:"pendingPayout,omitempty"`
	RequestedCheck int32    `json:"requestedCheck,omitempty"`
}

func getPendingChecks(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var influencers []*GreedyInfluencer
		for _, inf := range s.auth.Influencers.GetAll() {
			if inf.RequestedCheck > 0 {
				tmpGreedy := &GreedyInfluencer{
					Id:             inf.Id,
					Name:           inf.Name,
					Address:        inf.Address,
					PendingPayout:  inf.PendingPayout,
					RequestedCheck: inf.RequestedCheck,
					CompletedDeals: inf.GetPostURLs(inf.LastCheck),
					Followers:      inf.GetFollowers(),
					SigID:          inf.SignatureId,
				}
				influencers = append(influencers, tmpGreedy)
			}
		}
		c.JSON(200, influencers)
	}
}

func getPendingCampaigns(s *Server) gin.HandlerFunc {
	// Have we received the perks from advertiser? Are there any campaigns that have NOT
	// been approved by admin yet?
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

type PerkWithCmpInfo struct {
	DealID       string `json:"dealID"`
	InfluencerID string `json:"infID"`
	AdvertiserID string `json:"advID"`
	CampaignID   string `json:"cmpID"`
	CampaignName string `json:"cmpName"`
	Doc          string `json:"doc"` // HTML Representation of printout
	*common.Perk
}

func getPendingPerks(s *Server) gin.HandlerFunc {
	// Get list of perks that need to be mailed out
	return func(c *gin.Context) {
		var perks []PerkWithCmpInfo
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				for _, d := range cmp.Deals {
					if d.IsActive() && d.Perk != nil && !d.Perk.Status {
						perks = append(perks, PerkWithCmpInfo{
							DealID:       d.Id,
							InfluencerID: d.InfluencerId,
							AdvertiserID: cmp.AdvertiserId,
							CampaignID:   cmp.Id,
							CampaignName: cmp.Name,
							Perk:         d.Perk,
							Doc:          getPerkHandout(d, &cmp),
						})
					}
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, perks)
	}
}

var ErrPayout = errors.New("Nothing to payout!")

func approveCheck(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Delete the check and entry, send to lob
		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if inf.RequestedCheck == 0 || inf.PendingPayout == 0 {
			c.JSON(500, misc.StatusErr(ErrPayout.Error()))
			return
		}

		check, err := lob.CreateCheck(inf.Id, inf.Name, inf.Address, inf.PendingPayout, s.Cfg)
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		inf.Payouts = append(inf.Payouts, check)
		inf.PendingPayout = 0
		inf.RequestedCheck = 0
		inf.LastCheck = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if err := inf.CheckEmail(check, s.Cfg); err != nil {
			s.Alert("Failed to email check information to influencer "+inf.Id, err)
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func approveCampaign(s *Server) gin.HandlerFunc {
	// Used once we have approved the campaign!
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

func approvePerk(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		cid := c.Param("campaignId")
		if cid == "" {
			c.JSON(500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		for _, d := range inf.ActiveDeals {
			if d.CampaignId == cid && d.Perk != nil {
				d.Perk.Status = true
				d.PerkIncr()
				inf.PerkNotify(d, s.Cfg)
			}
		}

		if err := saveAllActiveDeals(s, inf); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

var (
	ErrSorry        = errors.New("Sorry! You are currently not eligible for a check!")
	ErrInvalidFunds = errors.New("Must have atleast $10 USD to be paid out!")
	ErrThirtyDays   = errors.New("Must wait atleast 30 days since last check to receive a payout!")
	ErrAddress      = errors.New("Please set an address for your profile!")
	ErrTax          = errors.New("Please fill out all necessary tax forms!")
)

const THIRTY_DAYS = 60 * 60 * 24 * 30 // Thirty days in seconds

func requestCheck(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Delete the check and entry, send to lob
		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		now := int32(time.Now().Unix())

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if inf.IsBanned() {
			c.JSON(500, misc.StatusErr(ErrSorry.Error()))
			return
		}

		if inf.PendingPayout < 10 {
			c.JSON(500, misc.StatusErr(ErrInvalidFunds.Error()))
			return
		}

		if inf.LastCheck > 0 && inf.LastCheck > now-THIRTY_DAYS {
			c.JSON(500, misc.StatusErr(ErrThirtyDays.Error()))
			return
		}

		if inf.Address == nil {
			c.JSON(500, misc.StatusErr(ErrAddress.Error()))
			return
		}

		if c.Query("skipTax") != "1" && !inf.HasSigned {
			c.JSON(500, misc.StatusErr(ErrTax.Error()))
			return
		}

		inf.RequestedCheck = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Check requested!", fmt.Sprintf("%s just requested a check of %f! Please check admin dash.", inf.Name, inf.PendingPayout))

		// Insert log
		c.JSON(200, misc.StatusOK(infId))
	}
}

var ErrDealNotFound = errors.New("Deal not found!")

func forceApproveAny(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		// Delete the check and entry, send to lob
		infId := c.Param("influencerId")
		campaignId := c.Param("campaignId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		var found *common.Deal
		for _, deal := range inf.ActiveDeals {
			if deal.CampaignId == campaignId {
				found = deal
			}
		}
		if found == nil {
			c.JSON(500, misc.StatusErr(ErrDealNotFound.Error()))
			return
		}

		var err error
		for _, pf := range found.Platforms {
			switch pf {
			case platform.Twitter:
				if inf.Twitter != nil && len(inf.Twitter.LatestTweets) > 0 {
					if err = s.ApproveTweet(inf.Twitter.LatestTweets[0], found); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
					break
				}
			case platform.Facebook:
				if inf.Facebook != nil && len(inf.Facebook.LatestPosts) > 0 {
					if err = s.ApproveFacebook(inf.Facebook.LatestPosts[0], found); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
					break
				}
			case platform.Instagram:
				if inf.Instagram != nil && len(inf.Instagram.LatestPosts) > 0 {
					if err = s.ApproveInstagram(inf.Instagram.LatestPosts[0], found); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
					break
				}
			case platform.YouTube:
				if inf.YouTube != nil && len(inf.YouTube.LatestPosts) > 0 {
					if err = s.ApproveYouTube(inf.YouTube.LatestPosts[0], found); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
					break
				}
			}
		}
		c.JSON(200, misc.StatusOK(infId))

	}
}

type ForceApproval struct {
	URL          string `json:"url,omitempty"`
	Platform     string `json:"platform,omitempty"`
	InfluencerID string `json:"infId,omitempty"`
	CampaignID   string `json:"campaignId,omitempty"`
}

func forceApprovePost(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// /platform/influencerId/campaignId/URL
		if !isSecureAdmin(c, s) {
			return
		}

		var fApp ForceApproval

		defer c.Request.Body.Close()
		if err := json.NewDecoder(c.Request.Body).Decode(&fApp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		postUrl := fApp.URL
		if postUrl == "" {
			c.JSON(400, misc.StatusErr("invalid post url"))
			return
		}

		infId := fApp.InfluencerID
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		campaignId := fApp.CampaignID
		cmp, ok := s.Campaigns.Get(campaignId)
		if !ok {
			c.JSON(500, misc.StatusErr("invalid campaign id"))
			return
		}

		if !cmp.IsValid() {
			c.JSON(500, misc.StatusErr("invalid campaign"))
			return
		}

		var foundDeal *common.Deal
		for _, deal := range cmp.Deals {
			if deal.IsAvailable() {
				foundDeal = deal
				break
			}
		}

		if foundDeal == nil {
			c.JSON(500, misc.StatusErr("no available deals left for this campaign"))
			return
		}

		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, campaignId, "")
		if err != nil || store == nil && store.Spendable == 0 && store.Spent > store.Budget {
			c.JSON(500, misc.StatusErr("campaign has no spendable left"))
			return
		}

		// Fill in some display properties for the deal
		// (Set in influencer.GetAvailableDeals otherwise)
		foundDeal.CampaignName = cmp.Name
		foundDeal.CampaignImage = cmp.ImageURL
		foundDeal.Company = cmp.Company
		foundDeal.InfluencerId = infId
		foundDeal.InfluencerName = inf.Name
		foundDeal.Assigned = int32(time.Now().Unix())

		// NOTE: Not touching the campaigns perks! Look into this

		// Update the influencer
		switch fApp.Platform {
		case platform.Twitter:
			if inf.Twitter == nil {
				c.JSON(500, misc.StatusErr("Influencer does not have this platform"))
				return
			}
			if err = inf.Twitter.UpdateData(s.Cfg, true); err != nil {
				c.String(400, err.Error())
				return
			}

			for _, post := range inf.Twitter.LatestTweets {
				if post.PostURL == postUrl {
					// So we just found the post.. lets accept!
					if err = s.ApproveTweet(post, foundDeal); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}
			}
		case platform.Instagram:
			if inf.Instagram == nil {
				c.JSON(500, misc.StatusErr("Influencer does not have this platform"))
				return
			}
			if err = inf.Instagram.UpdateData(s.Cfg, true); err != nil {
				c.String(400, err.Error())
				return
			}

			for _, post := range inf.Instagram.LatestPosts {
				if post.PostURL == postUrl {
					// So we just found the post.. lets accept!
					if err = s.ApproveInstagram(post, foundDeal); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}
			}
		case platform.YouTube:
			if inf.YouTube == nil {
				c.JSON(500, misc.StatusErr("Influencer does not have this platform"))
				return
			}
			if err = inf.YouTube.UpdateData(s.Cfg, true); err != nil {
				c.String(400, err.Error())
				return
			}

			for _, post := range inf.YouTube.LatestPosts {
				if post.PostURL == postUrl {
					// So we just found the post.. lets accept!
					if err = s.ApproveYouTube(post, foundDeal); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}
			}
		case platform.Facebook:
			if inf.Facebook == nil {
				c.JSON(500, misc.StatusErr("Influencer does not have this platform"))
				return
			}
			if err = inf.Facebook.UpdateData(s.Cfg, true); err != nil {
				c.String(400, err.Error())
				return
			}

			for _, post := range inf.Facebook.LatestPosts {
				if post.PostURL == postUrl {
					// So we just found the post.. lets accept!
					if err = s.ApproveFacebook(post, foundDeal); err != nil {
						c.JSON(500, misc.StatusErr(err.Error()))
						return
					}
				}
			}
		default:
			c.String(400, "Invalid platform")
			return
		}

		c.JSON(200, misc.StatusOK(infId))
		return
	}
}

func forceDeplete(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		if _, err := depleteBudget(s); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func forceTimeline(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if !cmp.Status {
					cmp.AddToTimeline(common.CAMPAIGN_PAUSED, false, s.Cfg)
				}

				time.Sleep(1 * time.Second)

				if cmp.HasMailedPerk() {
					cmp.AddToTimeline(common.PERKS_MAILED, true, s.Cfg)
				}

				time.Sleep(1 * time.Second)

				if cmp.HasAcceptedDeal() {
					cmp.AddToTimeline(common.DEAL_ACCEPTED, true, s.Cfg)
				}

				time.Sleep(1 * time.Second)

				if cmp.HasCompletedDeal() {
					cmp.AddToTimeline(common.CAMPAIGN_SUCCESS, true, s.Cfg)
				}

				saveCampaign(tx, &cmp, s)
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func forceEngine(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		if s.Cfg.Sandbox {
			if err := run(s); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
		} else {
			go run(s)
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func emailTaxForm(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Delete the check and entry, send to lob
		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if inf.SignatureId != "" {
			c.JSON(500, misc.StatusErr("Tax documents have already been sent! Please fill those out and allow us 4-8 hours to approve your information. Thank-you!"))
			return
		}

		if inf.Address == nil {
			c.JSON(500, misc.StatusErr(ErrAddress.Error()))
			return
		}

		sigId, err := hellosign.SendSignatureRequest(inf.Name, inf.EmailAddress, inf.Id, inf.IsAmerican(), s.Cfg.Sandbox)
		if err != nil {
			s.Alert("Hellosign signature request failed for "+inf.Id, err)
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		inf.SignatureId = sigId
		inf.RequestedTax = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		// Insert log
		c.JSON(200, misc.StatusOK(infId))
	}
}

func userProfile(srv *Server) gin.HandlerFunc {
	checkAdAgency := srv.auth.CheckOwnership(auth.AdAgencyItem, "id")
	checkTalentAgency := srv.auth.CheckOwnership(auth.TalentAgencyItem, "id")

	return func(c *gin.Context) {
		cu := auth.GetCtxUser(c)
		id := c.Param("id")

		if id == "" || id == cu.ID {
			goto SKIP
		}

		switch {
		case cu.Admin:
		case cu.AdAgency != nil:
			checkAdAgency(c)
		case cu.TalentAgency != nil:
			checkTalentAgency(c)
		default:
			misc.AbortWithErr(c, http.StatusUnauthorized, auth.ErrUnauthorized)
		}
		if c.IsAborted() {
			return
		}

		if cu = srv.auth.GetUser(id); cu == nil {
			misc.AbortWithErr(c, http.StatusNotFound, auth.ErrInvalidUserID)
		}

	SKIP:
		cu = cu.Trim()

		if cu.Advertiser == nil { // return the user if it isn't an advertiser
			c.JSON(200, cu)
			return
		}

		var advWithCampaigns struct {
			*auth.User
			HasCampaigns bool `json:"hasCmps"`
		}

		advWithCampaigns.User = cu

		srv.db.View(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(srv.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp struct {
					AdvertiserId string `json:"advertiserId"`
				}
				if json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				// if the campaign's adv id is the same as this user it means he has at least one cmp
				// set the flag and break the foreach early
				if cmp.AdvertiserId == cu.ID {
					advWithCampaigns.HasCampaigns = true
					return io.EOF
				}
				return
			})
		})

		c.JSON(200, &advWithCampaigns)
	}
}

func forceEmail(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		_, err := emailDeals(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func getKeywords(s *Server) gin.HandlerFunc {
	// Get all keywords in the system
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"keywords": s.Keywords})
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
		c.JSON(200, gin.H{"influencers": influencers, "reach": reach})
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

var ErrID = errors.New("Invalid click URL")

func click(s *Server) gin.HandlerFunc {
	domain := s.Cfg.Domain
	return func(c *gin.Context) {
		var (
			id = c.Param("id")
			v  []byte
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.URL)).Get([]byte(id))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(ErrID.Error()))
			return
		}

		parts := strings.Split(string(v), "::")
		if len(parts) != 2 {
			c.JSON(500, misc.StatusErr(ErrID.Error()))
			return
		}

		campaignId := parts[0]
		dealId := parts[1]

		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, misc.StatusErr(ErrCampaign.Error()))
			return
		}

		foundDeal, ok := cmp.Deals[dealId]
		if !ok || foundDeal == nil || foundDeal.Link == "" {
			c.JSON(500, misc.StatusErr(ErrDealNotFound.Error()))
			return
		}

		infId := foundDeal.InfluencerId
		// Stored as a comma separated list of dealIDs satisfied
		prevClicks := misc.GetCookie(c.Request, "click")
		if strings.Contains(prevClicks, foundDeal.Id) && c.Query("dbg") != "1" {
			// This user has already clicked once for this deal!
			c.Redirect(302, foundDeal.Link)
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			log.Println("Influencer not found for click!", infId, campaignId)
			c.Redirect(302, foundDeal.Link)
			return
		}

		var added bool

		// Lets search in completed deals first!
		for _, infDeal := range inf.CompletedDeals {
			if foundDeal.Id == infDeal.Id {
				infDeal.Click()
				added = true
				break
			}
		}

		if !added {
			// Ok lets check active deals if deal wasn't found in completed!
			for _, infDeal := range inf.ActiveDeals {
				if foundDeal.Id == infDeal.Id {
					infDeal.Click()
					added = true
					break
				}
			}
		}

		// SAVE!
		// Also saves influencers!
		if added {
			if err := saveAllDeals(s, inf); err != nil {
				c.Redirect(302, foundDeal.Link)
				return
			}

			if prevClicks != "" {
				prevClicks += "," + foundDeal.Id
			} else {
				prevClicks += foundDeal.Id
			}

			// One click per 30 days allowed per deal!
			misc.SetCookie(c.Writer, domain, "click", prevClicks, !s.Cfg.Sandbox, 24*30*time.Hour)
		}

		c.Redirect(302, foundDeal.Link)
	}
}

type FeedCell struct {
	Username     string `json:"username,omitempty"`
	InfluencerID string `json:"infID,omitempty"`
	URL          string `json:"url,omitempty"`
	Caption      string `json:"caption,omitempty"`

	CampaignID   string `json:"campaignID,omitempty"`
	CampaignName string `json:"campaignName,omitempty"`

	Published int32 `json:"published,omitempty"`

	Views    int32 `json:"views,omitempty"`
	Likes    int32 `json:"likes,omitempty"`
	Clicks   int32 `json:"clicks,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`

	Bonus bool `json:"bonus,omitempty"`

	// Links to a DP for the social media profile
	SocialImage string `json:"socialImage,omitempty"`
}

func (d *FeedCell) UseTweet(tw *twitter.Tweet) {
	d.Caption = tw.Text
	d.Published = int32(tw.CreatedAt.Unix())
	d.URL = tw.PostURL
}

func (d *FeedCell) UseInsta(insta *instagram.Post) {
	d.Caption = insta.Caption
	d.Published = insta.Published
	d.URL = insta.PostURL
}

func (d *FeedCell) UseFB(fb *facebook.Post) {
	d.Caption = fb.Caption
	d.Published = int32(fb.Published.Unix())
	d.URL = fb.PostURL
}

func (d *FeedCell) UseYT(yt *youtube.Post) {
	d.Caption = yt.Description
	d.Published = yt.Published
	d.URL = yt.PostURL
}

func getAdvertiserContentFeed(s *Server) gin.HandlerFunc {
	// Retrieves all completed deals by advertiser
	return func(c *gin.Context) {
		adv := s.auth.GetAdvertiser(c.Param("id"))
		if adv == nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		var feed []FeedCell
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == adv.ID {
					for _, deal := range cmp.Deals {
						if deal.Completed > 0 {
							d := FeedCell{
								CampaignID:   cmp.Id,
								CampaignName: cmp.Name,
								Username:     deal.InfluencerName,
								InfluencerID: deal.InfluencerId,
							}

							total := deal.TotalStats()
							d.Likes = total.Likes
							d.Comments = total.Comments
							d.Shares = total.Shares
							d.Views = total.Views
							d.Clicks = total.GetClicks()

							inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
							if !ok {
								log.Println("Influencer not found!", deal.InfluencerId)
								continue
							}

							if deal.Tweet != nil {
								d.UseTweet(deal.Tweet)
								if inf.Twitter != nil {
									d.SocialImage = inf.Twitter.ProfilePicture
								}
							} else if deal.Facebook != nil {
								d.UseFB(deal.Facebook)
								if inf.Facebook != nil {
									d.SocialImage = inf.Facebook.ProfilePicture
								}
							} else if deal.Instagram != nil {
								d.UseInsta(deal.Instagram)
								if inf.Instagram != nil {
									d.SocialImage = inf.Instagram.ProfilePicture
								}
							} else if deal.YouTube != nil {
								d.UseYT(deal.YouTube)
								if inf.YouTube != nil {
									d.SocialImage = inf.YouTube.ProfilePicture
								}
							}

							feed = append(feed, d)

							// Lets add extra cells for any bonus posts
							if deal.Bonus != nil {
								d.Bonus = true
								// Lets copy the cell so we can re-use values!
								for _, tw := range deal.Bonus.Tweet {
									dupeCell := d
									dupeCell.UseTweet(tw)
									dupeCell.Likes = int32(tw.Favorites)
									dupeCell.Comments = 0
									dupeCell.Shares = int32(tw.Retweets)
									dupeCell.Views = 0
									dupeCell.Clicks = 0

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.Facebook {
									dupeCell := d
									dupeCell.UseFB(post)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = int32(post.Shares)
									dupeCell.Views = 0
									dupeCell.Clicks = 0

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.Instagram {
									dupeCell := d
									dupeCell.UseInsta(post)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = 0
									dupeCell.Views = 0
									dupeCell.Clicks = 0

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.YouTube {
									dupeCell := d
									dupeCell.UseYT(post)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = 0
									dupeCell.Views = int32(post.Views)
									dupeCell.Clicks = 0

									feed = append(feed, dupeCell)
								}
							}
						}
					}
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, feed)
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

func advertiserBan(s *Server) gin.HandlerFunc {
	// Retrieves all completed deals by advertiser
	return func(c *gin.Context) {
		id := c.Param("id")
		adv := s.auth.GetAdvertiser(id)
		if adv == nil {
			c.JSON(500, misc.StatusErr("Please provide a valid advertiser"))
			return
		}

		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("Please provide a valid influencer"))
			return
		}

		user := auth.GetCtxUser(c)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if id != user.ID {
				user = s.auth.GetUserTx(tx, id)
			}
			if user == nil {
				return auth.ErrInvalidID
			}

			if len(adv.Blacklist) == 0 {
				adv.Blacklist = make(map[string]bool)
			}
			adv.Blacklist[infId] = true
			return user.StoreWithData(s.auth, tx, adv)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Copy the new blacklist to all campaigns under the advertiser!
		if err := s.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return err
				}

				if cmp.AdvertiserId == adv.ID {
					cmp.Blacklist = adv.Blacklist
				}

				if err = saveCampaign(tx, cmp, s); err != nil {
					log.Println("Error saving campaign for adv blacklist", err)
					return err
				}

				return nil
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}

type SimpleActive struct {
	CampaignId   string `json:"campaignId"`
	InfluencerId string `json:"influencerId,omitempty"`

	Platforms []string `json:"platforms,omitempty"`

	Facebook  string `json:"fbUsername,omitempty"`
	Instagram string `json:"instaUsername,omitempty"`
	Twitter   string `json:"twitterUsername,omitempty"`
	YouTube   string `json:"youtubeUsername,omitempty"`
}

func getAllActiveDeals(s *Server) gin.HandlerFunc {
	// Retrieves all active deals in the system
	return func(c *gin.Context) {
		if c.Query("simple") != "" {
			var deals []*SimpleActive
			if err := s.db.View(func(tx *bolt.Tx) error {
				tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
					var cmp common.Campaign
					if err := json.Unmarshal(v, &cmp); err != nil {
						log.Println("error when unmarshalling campaign", string(v))
						return nil
					}
					for _, deal := range cmp.Deals {
						if deal.IsActive() {
							inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
							if !ok {
								continue
							}

							infClean := inf.Clean()

							deals = append(deals, &SimpleActive{
								CampaignId:   cmp.Id,
								InfluencerId: deal.InfluencerId,
								Platforms:    deal.Platforms,
								Facebook:     infClean.FbUsername,
								Instagram:    infClean.InstaUsername,
								Twitter:      infClean.TwitterUsername,
								YouTube:      infClean.YTUsername,
							})
						}
					}
					return
				})
				return nil
			}); err != nil {
				c.JSON(500, misc.StatusErr("Internal error"))
				return
			}

			c.JSON(200, deals)
		} else {
			var deals []*common.Deal
			if err := s.db.View(func(tx *bolt.Tx) error {
				tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
					var cmp common.Campaign
					if err := json.Unmarshal(v, &cmp); err != nil {
						log.Println("error when unmarshalling campaign", string(v))
						return nil
					}
					for _, deal := range cmp.Deals {
						if deal.IsActive() {
							deals = append(deals, deal)
						}
					}
					return
				})
				return nil
			}); err != nil {
				c.JSON(500, misc.StatusErr("Internal error"))
				return
			}

			c.JSON(200, deals)
		}
	}
}

func setScrap(s *Server) gin.HandlerFunc {
	// Ingests a scrap and puts it into pool
	return func(c *gin.Context) {
		var (
			scraps []*influencer.Scrap
			err    error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&scraps); err != nil || len(scraps) == 0 {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err := saveScraps(s, scraps); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func optoutScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		scraps, _ := getAllScraps(s)
		for _, sc := range scraps {
			if sc.EmailAddress == email {
				sc.Ignore = true
				saveScrap(s, sc)
			}
		}

		c.String(200, "You have successfully been opted out. You may now close this window.")

	}
}

func getScraps(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		scraps, err := getAllScraps(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, scraps)
	}
}

func forceScrapEmail(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		count, err := emailScraps(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, gin.H{"count": count})
	}
}

func forceAttributer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		count, err := attributer(s, true)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, gin.H{"count": count})
	}
}

func scrapStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		scraps, err := getAllScraps(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		type ScrapStats struct {
			HasKeywords   int64 `json:"hasKeywords"`
			HasGeo        int64 `json:"hasGeo"`
			HasGender     int64 `json:"hasGender"`
			HasCategories int64 `json:"hasCategories"`
			Attributed    int64 `json:"attributed"`
			Total         int   `json:"total"`
		}

		stats := ScrapStats{Total: len(scraps)}
		for _, sc := range scraps {
			if sc.Attributed {
				stats.Attributed += 1
			}

			if len(sc.Keywords) > 0 {
				stats.HasKeywords += 1
			}

			if len(sc.Categories) > 0 {
				stats.HasCategories += 1
			}

			if sc.Geo != nil {
				stats.HasGeo += 1
			}

			if sc.Male || sc.Female {
				stats.HasGender += 1
			}
		}

		c.JSON(200, stats)
	}
}

func unapproveDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// NOTE: this does not add money back to spendable

		if !isSecureAdmin(c, s) {
			return
		}

		infId := c.Param("influencerId")
		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		dealId := c.Param("dealId")
		var d *common.Deal
		for _, deal := range inf.CompletedDeals {
			if deal.Id == dealId {
				d = deal
			}
		}

		if d == nil {
			c.JSON(500, misc.StatusErr("Invalid deal!"))
			return
		}

		// Marks the deal as INCOMPLETE, and updates the campaign and influencer buckets
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var (
				cmp *common.Campaign
			)

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(d.CampaignId)), &cmp)
			if err != nil {
				log.Println("Error unmarshallign campaign", err)
				return err
			}

			d = d.ConvertToActive()

			stats := d.TotalStats()
			inf.PendingPayout = inf.PendingPayout - stats.Influencer

			d.Reporting = nil
			cmp.Deals[d.Id] = d

			inf, ok := s.auth.Influencers.Get(d.InfluencerId)
			if !ok {
				log.Println("Error unmarshalling influencer")
				return ErrUnmarshal
			}

			// Add to completed deals
			if inf.ActiveDeals == nil || len(inf.ActiveDeals) == 0 {
				inf.ActiveDeals = []*common.Deal{}
			}
			inf.ActiveDeals = append(inf.ActiveDeals, d)

			// Remove from complete deals
			complDeals := []*common.Deal{}
			for _, deal := range inf.CompletedDeals {
				if deal.Id != d.Id {
					complDeals = append(complDeals, deal)
				}
			}
			inf.CompletedDeals = complDeals

			// Save the Influencer
			if err := saveInfluencer(s, tx, inf); err != nil {
				log.Println("Error saving influencer!", err)
				return err
			}

			// Save the campaign!
			if err := saveCampaign(tx, cmp, s); err != nil {
				log.Println("Error saving campaign!", err)
				return err
			}

			return nil
		}); err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

func influencerValue(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		ip := c.Query("ip")
		if ip == "" {
			c.String(400, "Invalid request")
			return
		}

		if waitingPeriod, ok := s.LimitSet.IsAllowed(ip); !ok {
			c.String(400, "Too many requests! Please wait "+waitingPeriod+" before trying again.")
			return
		}

		handle := c.Param("handle")
		if handle == "" {
			c.String(400, "Invalid social media handle")
			return
		}

		s.LimitSet.Set(ip)

		var value float64
		switch c.Param("platform") {
		case platform.Twitter:
			tw, err := twitter.New(handle, s.Cfg)
			if err != nil {
				c.String(400, err.Error())
				return
			}
			value += tw.AvgLikes * budget.TW_FAVORITE
			value += tw.AvgRetweets * budget.TW_RETWEET
		case platform.Instagram:
			insta, err := instagram.New(handle, s.Cfg)
			if err != nil {
				c.String(400, err.Error())
				return
			}
			value += insta.AvgLikes * budget.INSTA_LIKE
			value += insta.AvgComments * budget.INSTA_COMMENT
		case platform.YouTube:
			yt, err := youtube.New(handle, s.Cfg)
			if err != nil {
				c.String(400, err.Error())
				return
			}
			value += yt.AvgViews * budget.YT_VIEW
			value += yt.AvgComments * budget.YT_COMMENT
			value += yt.AvgLikes * budget.YT_LIKE
			value += yt.AvgDislikes * budget.YT_DISLIKE
		case platform.Facebook:
			fb, err := facebook.New(handle, s.Cfg)
			if err != nil {
				c.String(400, err.Error())
				return
			}
			value += fb.AvgLikes * budget.FB_LIKE
			value += fb.AvgComments * budget.FB_COMMENT
			value += fb.AvgShares * budget.FB_SHARE
		default:
			c.String(400, "Invalid platform")
			return
		}

		// Not factoring in margins for now
		// _, _, _, inf := budget.GetMargins(value, -1, -1, -1)

		c.String(200, strconv.FormatFloat(value, 'f', 6, 64))
		return
	}
}

func syncAllStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, inf := range s.auth.Influencers.GetAll() {
			for _, deal := range inf.CompletedDeals {
				if deal.IsComplete() {
					// Lets make sure numbers for likes and comments on insta
					// post line up with daily stats
					if deal.Instagram != nil {
						totalLikes := int32(deal.Instagram.Likes)
						totalComments := int32(deal.Instagram.Comments)

						var (
							reportingLikes, reportingComments int32
							key                               string
							stats                             *common.Stats
							highestLikes                      int32
						)

						for day, target := range deal.Reporting {
							if target.Likes > highestLikes {
								highestLikes = target.Likes

								stats = target
								key = day
							}
							reportingLikes += target.Likes
							reportingComments += target.Comments
						}

						if stats == nil || key == "" {
							continue
						}

						likesDiff := reportingLikes - totalLikes
						if likesDiff > 0 {
							// Subtract likes from stats

							if stats.Likes > likesDiff {
								// We have all the likes we need on 31st.. lets surtact!
								log.Println("Need to take out likes:", deal.Id, likesDiff)
								stats.Likes -= likesDiff
							}
						} else if likesDiff < 0 {
							// meaning we need to ADD likes
							stats.Likes += totalLikes - reportingLikes
						}

						commentsDiff := reportingComments - totalComments
						if commentsDiff > 0 {
							// Subtract comments from stats

							if stats.Comments >= commentsDiff {
								log.Println("Need to take out comments:", deal.Id, commentsDiff)

								// We have all the likes we need on 31st.. lets surtact!
								stats.Comments -= commentsDiff
							} else if commentsDiff < 0 {
								stats.Comments += totalComments - reportingComments
							}
						}

						// Save and bail
						deal.Reporting[key] = stats
					}
				}
			}
			saveAllCompletedDeals(s, inf)
		}

		c.JSON(200, misc.StatusOK(""))
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

func getTotalClicks(s *Server) gin.HandlerFunc {
	// Return all clicks that happened in the last X hours
	return func(c *gin.Context) {
		hours, _ := strconv.Atoi(c.Param("hours"))
		if hours == 0 {
			c.String(400, "Invalid hours value")
			return
		}

		total := make(map[string]int)

		for _, inf := range s.auth.Influencers.GetAll() {
			for _, deal := range inf.CompletedDeals {
				for _, stats := range deal.Reporting {
					for _, cl := range stats.ApprovedClicks {
						if misc.WithinLast(cl.TS, int32(hours)) {
							total[deal.CampaignId] += 1
						}
					}
				}
			}
		}

		c.JSON(200, total)
	}
}

func getServerStats(s *Server) gin.HandlerFunc {
	// Returns stored server stats
	return func(c *gin.Context) {
		c.JSON(200, s.Stats.Get())
	}
}
