package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/google"
	"github.com/swayops/sway/platforms/hellosign"
	"github.com/swayops/sway/platforms/lob"
)

///////// Talent Agencies ///////////
func putTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ag   auth.TalentAgency
			user = auth.GetCtxUser(c)
			id   = c.Param("id")
		)

		if err := c.BindJSON(&ag); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			if id != user.ID {
				user = s.auth.GetUserTx(tx, id)
			}
			if user == nil {
				return auth.ErrInvalidID
			}
			return user.StoreWithData(s.auth, tx, &ag)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}
		c.JSON(200, misc.StatusOK(id))
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
		SubCount int `json:"subCount"`
	}
	return func(c *gin.Context) {
		var (
			all    []*userWithCounts
			counts map[string]int
			uids   []string
		)

		s.db.View(func(tx *bolt.Tx) error {
			s.auth.GetUsersByTypeTx(tx, auth.TalentAgencyScope, func(u *auth.User) error {
				if u.TalentAgency != nil {
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

///////// Ad Agencies /////////
func putAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ag   auth.AdAgency
			user = auth.GetCtxUser(c)
			id   = c.Param("id")
		)

		if err := c.BindJSON(&ag); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			if id != user.ID {
				user = s.auth.GetUserTx(tx, id)
			}
			if user == nil {
				return auth.ErrInvalidID
			}
			return user.StoreWithData(s.auth, tx, &ag)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
		}
		c.JSON(200, misc.StatusOK(id))
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

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			adv  auth.User
			user = auth.GetCtxUser(c)
			id   = c.Param("id")
		)

		if err := c.BindJSON(&adv); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

		if adv.Advertiser == nil {
			misc.AbortWithErr(c, 400, auth.ErrInvalidRequest)
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) error {
			if id != user.ID {
				user = s.auth.GetUserTx(tx, id)
			}
			if user == nil {
				return auth.ErrInvalidID
			}
			return user.Update(&adv).StoreWithData(s.auth, tx, adv.Advertiser)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
		}

		c.JSON(200, misc.StatusOK(id))
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

		if cmp.Budget <= 0 {
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
			cmp.Tags[i] = sanitizeHash(ht)
		}

		cmp.Link = sanitizeURL(cmp.Link)
		cmp.Mention = sanitizeMention(cmp.Mention)
		cmp.Categories = common.LowerSlice(cmp.Categories)
		cmp.Whitelist = common.TrimEmails(cmp.Whitelist)

		if len(adv.Blacklist) > 0 {
			// Blacklist is always set at the advertiser level using content feed bad!
			cmp.Blacklist = adv.Blacklist
		}

		// If there are perks.. campaign is in pending until admin approval
		if cmp.Perks != nil {
			cmp.Approved = 0
		} else {
			cmp.Approved = int32(time.Now().Unix())
		}

		cmp.CreatedAt = time.Now().Unix()

		if err = s.db.Update(func(tx *bolt.Tx) (err error) { // have to get an id early for saveImage
			cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign)
			return
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if cmp.ImageData != "" {
			if !strings.HasPrefix(cmp.ImageData, "data:image/") {
				c.JSON(400, misc.StatusErr("Please provide a valid campaign image"))
				return
			}
			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), cmp.ImageData, cmp.Id)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			cmp.ImageURL, cmp.ImageData = getImageUrl(s, s.Cfg.Bucket.Campaign, filename), ""
		} else {
			cmp.ImageURL = getImageUrl(s, s.Cfg.Bucket.Campaign, DEFAULT_IMAGES[rand.Intn(len(DEFAULT_IMAGES))])
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			// Create their budget key
			// NOTE: Create budget key requires cmp.Id be set
			var spendable float64
			dspFee, exchangeFee := getAdvertiserFees(s.auth, cmp.AdvertiserId)
			if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, &cmp, 0, 0, dspFee, exchangeFee, false); err != nil {
				log.Println("Error creating budget key!", err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			addDealsToCampaign(&cmp, spendable)
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Email eligible influencers!
		if cmp.Perks == nil {
			if len(cmp.Whitelist) > 0 {
				go func() {
					// Wait an hour before emailing
					time.Sleep(1 * time.Hour)
					emailList(s, cmp.Id, nil)
				}()
			} else {
				go func() {
					// Wait 2 hours before emailing
					time.Sleep(2 * time.Hour)
					emailDeal(s, cmp.Id)
				}()
			}
		}

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

		if c.Query("deals") != "true" {
			// Hide deals otherwise output will get massive
			cmp.Deals = nil
		}

		c.JSON(200, cmp)
	}
}

func getCampaignsByAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAdv := c.Param("id")
		var campaigns []*common.Campaign
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
					// No need to display massive deal set
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
	Status     *bool            `json:"status,omitempty"`
	Budget     *float64         `json:"budget,omitempty"`
	Male       *bool            `json:"male,omitempty"`
	Female     *bool            `json:"female,omitempty"`
	Name       *string          `json:"name,omitempty"`
	Whitelist  map[string]bool  `json:"whitelist,omitempty"`
	ImageData  string           `json:"imageData,omitempty"` // this is input-only and never saved to the db
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
			upd   CampaignUpdate
			added float64
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

			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, cmp.Id), upd.ImageData, cmp.Id)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			cmp.ImageURL, upd.ImageData = getImageUrl(s, s.Cfg.Bucket.Campaign, filename), ""
		}

		for _, g := range upd.Geos {
			if !geo.IsValidGeoTarget(g) {
				c.JSON(400, misc.StatusErr("Please provide valid geo targets!"))
				return
			}
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

		if upd.Budget != nil && cmp.Budget != *upd.Budget {
			// Update their budget!
			dspFee, exchangeFee := getAdvertiserFees(s.auth, cmp.AdvertiserId)
			if added, err = budget.AdjustBudget(s.budgetDb, s.Cfg, cmp.Id, *upd.Budget, dspFee, exchangeFee); err != nil {
				log.Println("Error creating budget key!", err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if added > 0 {
				addDealsToCampaign(&cmp, added)
			}

			cmp.Budget = *upd.Budget
		}

		if upd.Status != nil {
			cmp.Status = *upd.Status
		}
		cmp.Geos = upd.Geos
		cmp.Categories = common.LowerSlice(upd.Categories)

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
		if len(additions) > 0 {
			go emailList(s, cmp.Id, additions)
		}

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

///////// Influencers /////////
var (
	ErrBadGender = errors.New("Please provide a gender ('m' or 'f')")
	ErrNoAgency  = errors.New("Please provide an agency id")
	ErrNoGeo     = errors.New("Please provide a geo")
	ErrNoName    = errors.New("Please provide a name")
	ErrBadCat    = errors.New("Please provide a valid category")
)

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
				inf.Clean()
				st := reporting.GetInfluencerBreakdown(inf, s.Cfg, -1, inf.Rep, inf.CurrentRep, "", inf.AgencyId)
				total := st["total"]
				if total != nil {
					inf.AgencySpend = total.AgencySpent
					inf.InfluencerSpend = total.Spent
				}
				influencers = append(influencers, inf)
			}
		}
		c.JSON(200, influencers)
	}
}

var (
	ErrPlatform  = errors.New("Platform not found!")
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
)

func setPlatform(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Alter influencer bucket
		var (
			infId    = c.Param("influencerId")
			id       = c.Param("id")
			platform = c.Param("platform")
			err      error
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		switch platform {
		case "instagram":
			err = inf.NewInsta(id, s.Cfg)
		case "facebook":
			err = inf.NewFb(id, s.Cfg)
		case "twitter":
			err = inf.NewTwitter(id, s.Cfg)
		case "youtube":
			err = inf.NewYouTube(id, s.Cfg)
		default:
			c.JSON(500, misc.StatusErr(ErrPlatform.Error()))
			return
		}

		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cat := strings.ToLower(c.Param("category"))
		if _, ok := common.CATEGORIES[cat]; !ok {
			c.JSON(400, misc.StatusErr(ErrBadCat.Error()))
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
		inf.Categories = append(inf.Categories, cat)

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setInviteCode(s *Server) gin.HandlerFunc {
	// Sets the agency id for the influencer via an invite code
	return func(c *gin.Context) {
		agencyId := common.GetIDFromInvite(c.Params.ByName("inviteCode"))
		if agencyId == "" {
			agencyId = auth.SwayOpsTalentAgencyID
		}
		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			c.JSON(500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.AgencyId = agencyId

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setGender(s *Server) gin.HandlerFunc {
	// Sets the gender for the influencer id
	return func(c *gin.Context) {
		gender := strings.ToLower(c.Params.ByName("gender"))
		switch gender {
		case "m", "f", "mf", "fm", "unicorn":
		default:
			c.JSON(400, misc.StatusErr(ErrBadGender.Error()))
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
		switch gender {
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

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setReminder(s *Server) gin.HandlerFunc {
	// Sets the reminder for the influencer id
	return func(c *gin.Context) {
		state := strings.ToLower(c.Params.ByName("state"))

		var reminder bool
		if state == "t" || state == "true" {
			reminder = true
		} else if state == "f" || state == "false" {
			reminder = false
		} else {
			c.JSON(400, misc.StatusErr("Please submit a valid reminder state"))
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

		inf.DealPing = reminder

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setBan(s *Server) gin.HandlerFunc {
	// Sets the banned value for the influencer id
	return func(c *gin.Context) {
		state := strings.ToLower(c.Params.ByName("state"))

		var ban bool
		if state == "t" || state == "true" {
			ban = true
		} else if state == "f" || state == "false" {
			ban = false
		} else {
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

		c.JSON(200, misc.StatusOK(infId))
	}
}

func setAddress(s *Server) gin.HandlerFunc {
	// Sets the address for the influencer
	return func(c *gin.Context) {
		var (
			addr lob.AddressLoad
			err  error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&addr); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		cleanAddr, err := lob.VerifyAddress(&addr, s.Cfg.Sandbox)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		if !geo.IsValidGeo(&geo.GeoRecord{State: cleanAddr.State, Country: cleanAddr.Country}) {
			c.JSON(400, misc.StatusErr("Address does not convert to a valid geo!"))
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

		inf.Address = cleanAddr

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
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
		var influencers []*IncompleteInfluencer
		for _, inf := range s.auth.Influencers.GetAll() {
			if (!inf.Male && !inf.Female) || len(inf.Categories) == 0 {
				incInf := &IncompleteInfluencer{inf, "", "", "", ""}
				if inf.Twitter != nil {
					incInf.TwitterURL = "https://twitter.com/" + inf.Twitter.Id
				}
				if inf.Facebook != nil {
					incInf.FacebookURL = "https://www.facebook.com/" + inf.Facebook.Id
				}

				if inf.Instagram != nil {
					incInf.InstagramURL = "https://www.instagram.com/" + inf.Instagram.UserName
				}

				if inf.YouTube != nil {
					incInf.YouTubeURL = "https://www.youtube.com/user/" + inf.YouTube.UserName
				}
				influencers = append(influencers, incInf)
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

		deals := inf.GetAvailableDeals(s.Campaigns, s.budgetDb, "",
			geo.GetGeoFromCoords(lat, long, int32(time.Now().Unix())), false, s.Cfg)
		c.JSON(200, deals)
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
		currentDeals := inf.GetAvailableDeals(s.Campaigns, s.budgetDb, dealId, nil, true, s.Cfg)
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

		// Set the shortened URL for the influencer
		shortened, err := google.ShortenURL(getClickUrl(infId, foundDeal, s.Cfg), s.Cfg)
		if err != nil {
			c.JSON(500, misc.StatusErr("Internal error! Please try again in a few minutes"))
			return
		}
		foundDeal.ShortenedLink = shortened

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
					return errors.New("Please enter a valid address in your profile before accepting this deal")
				}

				// Now that we know there is a deal for this dude..
				// and they have an address.. schedule a perk order!

				cmp.Perks.Count -= 1
				foundDeal.Perk = &common.Perk{
					Name:     cmp.Perks.Name,
					Category: cmp.Perks.Category,
					Count:    1,
					InfId:    inf.Id,
					InfName:  inf.Name,
					Address:  inf.Address,
					Status:   false,
				}
			}

			foundDeal.InfluencerId = infId
			foundDeal.InfluencerName = inf.Name
			foundDeal.Assigned = int32(time.Now().Unix())

			foundPlatform := false
			for _, p := range foundDeal.Platforms {
				if p == mediaPlatform {
					foundPlatform = true
					break
				}
			}

			if !foundPlatform {
				return errors.New("Unforunately, the requested deal is no longer available!")
			}

			foundDeal.AssignedPlatform = mediaPlatform
			cmp.Deals[foundDeal.Id] = foundDeal

			// Append to the influencer's active deals
			inf.ActiveDeals = append(inf.ActiveDeals, foundDeal)

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

		c.JSON(200, inf.CleanAssignedDeals())
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

		c.JSON(200, inf.CleanCompletedDeals())
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
		c.JSON(200, store)
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

					if d.Assigned > 0 && d.Completed == 0 {
						dealsAccept += 1
					}

					if d.Assigned > 0 && d.Completed > 0 {
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
					clicks += stats.Clicks
				}
			}

			completionRate := 100 * (float64(dealsComplete-dealsAccept) / float64(dealsComplete))
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
			key = budget.GetCurrentBudgetKey()
		}

		// Now that it's a new month.. get last month's budget store
		store, err := budget.GetStore(s.budgetDb, s.Cfg, key)
		if err != nil || len(store) == 0 {
			// Insert file informant check
			c.JSON(500, misc.StatusErr(ErrEmptyStore))
			return
		}

		advertiserXf := misc.NewXLSXFile(s.Cfg.JsonXlsxPath)
		advertiserSheets := make(map[string]*misc.Sheet)

		agencyXf := misc.NewXLSXFile(s.Cfg.JsonXlsxPath)
		agencySheets := make(map[string]*misc.Sheet)

		// Agency Invoice
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

			if data.Spent == 0 {
				continue
			}

			if adAgency.ID == auth.SwayOpsAdAgencyID {
				// ADVERTISER INVOICE!
				sheet, ok := advertiserSheets[cmp.AdvertiserId]
				if !ok {
					sheet = advertiserXf.AddSheet(fmt.Sprintf("%s (%s)", advertiser.Name, advertiser.ID))
					sheet.AddHeader(
						"ID",
						"Name",
						"Email",
						"DSP Fee",
						"Exchange Fee",
						"Total Spent ($)",
					)
					advertiserSheets[cmp.AdvertiserId] = sheet
				}
				sheet.AddRow(
					cmp.Id,
					cmp.Name,
					emails,
					fmt.Sprintf("%0.2f", data.DspFee*100)+"%",
					fmt.Sprintf("%0.2f", data.ExchangeFee*100)+"%",
					misc.TruncateFloat(data.Spent, 2),
				)
			} else {
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
				sheet.AddRow(
					cmp.AdvertiserId,
					advertiser.Name,
					cmp.Id,
					cmp.Name,
					emails,
					fmt.Sprintf("%0.2f", data.DspFee*100)+"%",
					fmt.Sprintf("%0.2f", data.ExchangeFee*100)+"%",
					misc.TruncateFloat(data.Spent, 2),
				)
			}

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

		if len(advertiserSheets) > 0 {
			fName := fmt.Sprintf("%s-advertiser.xlsx", key)
			location := filepath.Join(s.Cfg.LogsPath, "invoices", fName)

			advo, err := os.Create(location)
			if err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if _, err := advertiserXf.WriteTo(advo); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}

			if err := advo.Close(); err != nil {
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

				// Transfer over budgets for anyone regardless of status
				// because they could set to active mid-month and would
				// not have the full month's budget they have set (since
				// they could have started the campaign mid-month and would
				// have only a portion of their monthly budget)
				if cmp.Budget > 0 {
					// This will carry over any left over spendable too
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
					dspFee, exchangeFee := getAdvertiserFeesFromTx(s.auth, tx, cmp.AdvertiserId)
					if spendable, err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, cmp, leftover, pending, dspFee, exchangeFee, true); err != nil {
						log.Println("Error creating budget key!", err)
						return err
					}

					// Add fresh deals for this month
					addDealsToCampaign(cmp, spendable)

					if err = saveCampaign(tx, cmp, s); err != nil {
						log.Println("Error saving campaign for billing", err)
						return err
					}
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

type GreedyInfluencer struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

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
				}
				influencers = append(influencers, tmpGreedy)
			}
		}
		c.JSON(200, influencers)
	}
}

func getPendingCampaigns(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var campaigns []*common.Campaign
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.Approved == 0 {
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

func getPendingPerks(s *Server) gin.HandlerFunc {
	// Get list of perks that need to be mailed out
	return func(c *gin.Context) {
		perks := make(map[string][]*common.Perk)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				for _, d := range cmp.Deals {
					if d.Perk != nil && !d.Perk.Status {
						perks[cmp.Id] = append(perks[cmp.Id], d.Perk)
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

		check, err := lob.CreateCheck(inf.Name, inf.Address, inf.PendingPayout, s.Cfg.Sandbox)
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

		c.JSON(200, misc.StatusOK(infId))
	}
}

func approveCampaign(s *Server) gin.HandlerFunc {
	// Used once we have received the perk!
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

		cmp.Approved = int32(time.Now().Unix())

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Email eligible influencers now that perks are approved!
		if len(cmp.Whitelist) > 0 {
			go func() {
				// Wait an hour before emailing
				time.Sleep(1 * time.Hour)
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
	ErrInvalidFunds = errors.New("Must have atleast $50 USD to be paid out!")
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

		if inf.Banned {
			c.JSON(500, misc.StatusErr(ErrSorry.Error()))
			return
		}

		if inf.PendingPayout < 50 {
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
		switch found.AssignedPlatform {
		case platform.Twitter:
			if inf.Twitter != nil && len(inf.Twitter.LatestTweets) > 0 {
				if err = s.ApproveTweet(inf.Twitter.LatestTweets[0], found); err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		case platform.Facebook:
			if inf.Facebook != nil && len(inf.Facebook.LatestPosts) > 0 {
				if err = s.ApproveFacebook(inf.Facebook.LatestPosts[0], found); err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		case platform.Instagram:
			if inf.Instagram != nil && len(inf.Instagram.LatestPosts) > 0 {
				if err = s.ApproveInstagram(inf.Instagram.LatestPosts[0], found); err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		case platform.YouTube:
			if inf.YouTube != nil && len(inf.YouTube.LatestPosts) > 0 {
				if err = s.ApproveYouTube(inf.YouTube.LatestPosts[0], found); err != nil {
					c.JSON(500, misc.StatusErr(err.Error()))
					return
				}
			}
		default:
			c.JSON(500, misc.StatusErr(ErrPlatform.Error()))
			return
		}
		c.JSON(200, misc.StatusOK(infId))

	}
}

func forceDeplete(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		if err := depleteBudget(s); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
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

		if err := run(s); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
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

		if inf.Address == nil {
			c.JSON(500, misc.StatusErr(ErrAddress.Error()))
			return
		}

		var isAmerican bool
		if strings.ToLower(inf.Address.Country) == "us" {
			isAmerican = true
		}

		sigId, err := hellosign.SendSignatureRequest(inf.Name, inf.EmailAddress, inf.Id, isAmerican, s.Cfg.Sandbox)
		if err != nil {
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
		switch {
		case cu.Admin:
		case cu.AdAgency != nil:
			checkAdAgency(c)
		case cu.TalentAgency != nil:
			checkTalentAgency(c)
		case cu.ID == id:
			c.JSON(200, cu.Trim())
			return
		default:
			misc.AbortWithErr(c, http.StatusUnauthorized, auth.ErrUnauthorized)
		}
		if c.IsAborted() {
			return
		}
		if cu = srv.auth.GetUser(id); cu != nil {
			c.JSON(200, cu.Trim())
		} else {
			misc.AbortWithErr(c, http.StatusNotFound, auth.ErrInvalidUserID)
		}

	}
}

func forceEmail(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		err := emailDeals(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(""))
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
		filename, err := saveImageToDisk(s.Cfg.ImagesDir+bucket+"/"+id, upd.Data, id)
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

			imageURL = getImageUrl(s, "campaign", filename)
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

func click(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			infId      = c.Param("influencerId")
			campaignId = c.Param("campaignId")
			dealId     = c.Param("dealId")
		)

		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)
		if cmp == nil {
			c.JSON(500, ErrCampaign)
			return
		}

		foundDeal, ok := cmp.Deals[dealId]
		if !ok || foundDeal == nil || foundDeal.Link == "" {
			c.JSON(500, ErrDealNotFound)
			return
		}

		if foundDeal.Completed == 0 {
			c.Redirect(302, foundDeal.Link)
			return
		}

		if foundDeal.InfluencerId != infId {
			c.Redirect(302, foundDeal.Link)
			return
		}

		// Stored as a comma separated list of dealIDs satisfied
		prevClicks := misc.GetCookie(c.Request, "click")
		if strings.Contains(prevClicks, foundDeal.Id) {
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

		for _, infDeal := range inf.CompletedDeals {
			if foundDeal.Id == infDeal.Id && infDeal.Completed > 0 {
				infDeal.Click()
				break
			}
		}

		// SAVE!
		// Also saves influencers!
		if err := saveAllCompletedDeals(s, inf); err != nil {
			c.Redirect(302, foundDeal.Link)
			return
		}

		if prevClicks != "" {
			prevClicks += "," + foundDeal.Id
		} else {
			prevClicks += foundDeal.Id
		}

		// One click per 30 days allowed per deal!
		misc.SetCookie(c.Writer, "click", prevClicks, 24*30*time.Hour)

		c.Redirect(302, foundDeal.Link)
	}
}

type FeedCell struct {
	Username     string `json:"username,omitempty"`
	InfluencerID string `json:"infID,omitempty"`
	URL          string `json:"url,omitempty"`
	Caption      string `json:"caption,omitempty"`

	Published int32 `json:"published,omitempty"`

	Views    int32 `json:"views,omitempty"`
	Likes    int32 `json:"likes,omitempty"`
	Clicks   int32 `json:"clicks,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`
}

func getAdvertiserContentFeed(s *Server) gin.HandlerFunc {
	// Retrieves all completed deals by advertiser
	return func(c *gin.Context) {
		adv := s.auth.GetAdvertiser(c.Param("id"))
		if adv == nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		var feed []*FeedCell
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
							d := &FeedCell{
								Username:     deal.InfluencerName,
								URL:          deal.PostUrl,
								InfluencerID: deal.InfluencerId,
							}

							total := deal.TotalStats()
							d.Likes = total.Likes
							d.Comments = total.Comments
							d.Shares = total.Shares
							d.Views = total.Views
							d.Clicks = total.Clicks

							if deal.Tweet != nil {
								d.Caption = deal.Tweet.Text
								d.Published = int32(deal.Tweet.CreatedAt.Unix())
								d.Views = reporting.GetViews(d.Likes, 0, d.Shares)
							} else if deal.Facebook != nil {
								d.Caption = deal.Facebook.Caption
								d.Published = int32(deal.Facebook.Published.Unix())
								d.Views = reporting.GetViews(d.Likes, d.Comments, d.Shares)
							} else if deal.Instagram != nil {
								d.Caption = deal.Instagram.Caption
								d.Published = deal.Instagram.Published
								d.Views = reporting.GetViews(d.Likes, d.Comments, 0)
							} else if deal.YouTube != nil {
								d.Caption = deal.YouTube.Description
								d.Published = deal.YouTube.Published
								d.Views = int32(deal.YouTube.Views)
							}
							feed = append(feed, d)
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

func getAllActiveDeals(s *Server) gin.HandlerFunc {
	// Retrieves all active deals in the system
	return func(c *gin.Context) {
		var deals []*common.Deal
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				for _, deal := range cmp.Deals {
					if deal.Assigned > 0 && deal.Completed == 0 {
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
