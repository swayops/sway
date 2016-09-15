package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
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
	return func(c *gin.Context) {
		c.JSON(200, getTalentAgencies(s))
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
	return func(c *gin.Context) {
		c.JSON(200, getAdAgencies(s))
	}
}

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			adv  auth.Advertiser
			user = auth.GetCtxUser(c)
			id   = c.Param("id")
		)

		if err := c.BindJSON(&adv); err != nil {
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
			return user.StoreWithData(s.auth, tx, &adv)
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
	return func(c *gin.Context) {
		targetAgency := c.Param("id")
		var advertisers []*auth.Advertiser
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.AdvertiserScope, func(u *auth.User) error {
				if adv := auth.GetAdvertiser(u); adv != nil && adv.AgencyID == targetAgency {
					advertisers = append(advertisers, adv)
				}
				return nil
			})
		})
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

		if cmp.Gender != "m" && cmp.Gender != "f" && cmp.Gender != "mf" {
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
		if cmp.Whitelist != nil {
			cmp.Whitelist = cmp.Whitelist.Sanitize()
		}

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

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign); err != nil {
				return
			}

			// Set the default image URL for starters to a random default image
			// OVERWRITTEN when uploadImage called
			if cmp.ImageURL == "" {
				cmp.ImageURL = getImageUrl(s, "campaign", DEFAULT_IMAGES[rand.Intn(len(DEFAULT_IMAGES))])
			}

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
			go emailDeal(s, &cmp)
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
	Geos       []*geo.GeoRecord   `json:"geos,omitempty"`
	Categories []string           `json:"categories,omitempty"`
	Status     bool               `json:"status,omitempty"`
	Budget     float64            `json:"budget,omitempty"`
	Gender     string             `json:"gender,omitempty"` // "m" or "f" or "mf"
	Name       string             `json:"name,omitempty"`
	Whitelist  *common.TargetList `json:"whitelist,omitempty"`
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
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		for _, g := range upd.Geos {
			if !geo.IsValidGeoTarget(g) {
				c.JSON(400, misc.StatusErr("Please provide valid geo targets!"))
				return
			}
		}

		if upd.Gender != "m" && upd.Gender != "f" && upd.Gender != "mf" {
			c.JSON(400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		if upd.Budget == 0 {
			c.JSON(400, misc.StatusErr("Please provide a valid budget"))
			return
		}
		if upd.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}
		cmp.Name = upd.Name

		if cmp.Budget != upd.Budget {
			// Update their budget!
			dspFee, exchangeFee := getAdvertiserFees(s.auth, cmp.AdvertiserId)
			if added, err = budget.AdjustBudget(s.budgetDb, s.Cfg, cmp.Id, upd.Budget, dspFee, exchangeFee); err != nil {
				log.Println("Error creating budget key!", err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
			if added > 0 {
				addDealsToCampaign(&cmp, added)
			}
			cmp.Budget = upd.Budget
		}

		cmp.Status = upd.Status
		cmp.Geos = upd.Geos
		cmp.Gender = upd.Gender
		cmp.Categories = common.LowerSlice(upd.Categories)

		if upd.Whitelist != nil {
			cmp.Whitelist = upd.Whitelist.Sanitize()
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
		inf := s.auth.GetInfluencer(c.Param("id"))
		if inf == nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, inf)
	}
}

func getInfluencersByCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetCat := c.Param("category")
		var influencers []*auth.Influencer
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
				inf := auth.GetInfluencer(u)
				if inf == nil {
					return nil
				}
				for _, infCat := range inf.Categories {
					if infCat == targetCat {
						inf.Clean()
						influencers = append(influencers, inf)
					}
				}
				return nil
			})
		})
		c.JSON(200, influencers)
	}
}

func getInfluencersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAg := c.Param("id")
		var influencers []*auth.Influencer
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
				inf := auth.GetInfluencer(u)
				if inf == nil {
					return nil
				}
				if inf.AgencyId == targetAg {
					inf.Clean()
					st := reporting.GetInfluencerBreakdown(inf.Id, s.auth, s.Cfg, -1, inf.Rep, inf.CurrentRep, "", inf.AgencyId)
					total := st["total"]
					if total != nil {
						inf.AgencySpend = total.AgencySpent
						inf.InfluencerSpend = total.Spent
					}

					influencers = append(influencers, inf)
				}
				return nil
			})
		})
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
			user     = auth.GetCtxUser(c)
		)

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
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
				return ErrPlatform
			}
			if err != nil {
				return
			}
			return user.StoreWithData(s.auth, tx, inf)
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
			user  = auth.GetCtxUser(c)
		)

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}
			inf.Categories = append(inf.Categories, cat)
			return user.StoreWithData(s.auth, tx, inf)
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
			user  = auth.GetCtxUser(c)
		)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			inf.AgencyId = agencyId

			return user.StoreWithData(s.auth, tx, inf)
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
		if gender != "m" && gender != "f" && gender != "unicorn" {
			c.JSON(400, misc.StatusErr(ErrBadGender.Error()))
			return
		}

		var (
			infId = c.Param("influencerId")
			user  = auth.GetCtxUser(c)
		)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			inf.Gender = gender

			return user.StoreWithData(s.auth, tx, inf)
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
			user  = auth.GetCtxUser(c)
		)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			inf.DealPing = reminder

			return user.StoreWithData(s.auth, tx, inf)
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
			user  = auth.GetCtxUser(c)
		)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			inf.Banned = ban

			return user.StoreWithData(s.auth, tx, inf)
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
			user  = auth.GetCtxUser(c)
		)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			inf.Address = cleanAddr

			// Save the influencer
			return user.StoreWithData(s.auth, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
	}
}

func getIncompleteInfluencers(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var influencers []*auth.Influencer
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
				if inf := auth.GetInfluencer(u); inf != nil {
					if inf.Gender == "" && len(inf.Categories) == 0 {
						influencers = append(influencers, inf)
					}
				}
				return nil
			})
		})
		c.JSON(200, influencers)
	}
}

func getCategories(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, common.GetCategories())
	}
}

///////// Deals /////////
func getDealsForInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			infId   = c.Param("influencerId")
			user    = auth.GetCtxUser(c)
			lat, _  = strconv.ParseFloat(c.Param("lat"), 64)
			long, _ = strconv.ParseFloat(c.Param("long"), 64)
			inf     *auth.Influencer
		)

		if len(infId) == 0 {
			c.JSON(500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		if err := s.db.View(func(tx *bolt.Tx) error {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}

			if inf = auth.GetInfluencer(user); inf == nil {
				return auth.ErrInvalidID
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
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
			user          = auth.GetCtxUser(c)
			dealId        = c.Param("dealId")
			campaignId    = c.Param("campaignId")
			mediaPlatform = c.Param("platform")
			inf           *auth.Influencer
		)

		if _, ok := platform.ALL_PLATFORMS[mediaPlatform]; !ok {
			c.JSON(500, misc.StatusErr("This platform was not found"))
			return
		}

		if err := s.db.View(func(tx *bolt.Tx) error {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}

			if inf = auth.GetInfluencer(user); inf == nil {
				return auth.ErrInvalidID
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
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
			if err = user.StoreWithData(s.auth, tx, inf); err != nil {
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
		var (
			inf *auth.Influencer
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			inf = s.auth.GetInfluencerTx(tx, c.Param("influencerId"))
			return nil
		}); err != nil || inf == nil {
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
		user := auth.GetCtxUser(c)
		if err := clearDeal(s, user, dealId, influencerId, campaignId, false); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(dealId))
	}
}

func getDealsCompletedByInfluencer(s *Server) gin.HandlerFunc {
	// Get all deals completed by the influencer in the last X hours
	return func(c *gin.Context) {
		inf := s.auth.GetInfluencer(c.Param("influencerId"))
		if inf == nil {
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

			talentAgencyCount := len(getTalentAgencies(s))
			var (
				infCount                               int
				reach                                  int64
				likes, comments, shares, views, clicks int32
				totalInfluencer, totalAgency           float64
			)

			s.db.View(func(tx *bolt.Tx) error {
				return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
					if inf := auth.GetInfluencer(u); inf != nil {
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
					return nil
				})
			})

			completionRate := 100 * (float64(dealsComplete-dealsAccept) / float64(dealsComplete))
			a = &AdminStats{
				AdAgencies:            len(getAdAgencies(s)),
				Advertisers:           len(getAdvertisers(s)),
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
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

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
					campaigns = append(campaigns, &cmp)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		cmpStats := []map[string]*reporting.Totals{}
		for _, cmp := range campaigns {
			stats := reporting.GetCampaignBreakdown(cmp.Id, s.db, s.Cfg, days)
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

		c.JSON(200, reporting.GetCampaignBreakdown(c.Param("cid"), s.db, s.Cfg, days))
	}
}

func getInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		infId := c.Param("influencerId")

		inf := s.auth.GetInfluencer(infId)
		if inf == nil {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}
		c.JSON(200, reporting.GetInfluencerBreakdown(infId, s.auth, s.Cfg, days, inf.Rep, inf.CurrentRep, "", ""))
	}
}

func getCampaignInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		infId := c.Param("infId")
		inf := s.auth.GetInfluencer(infId)
		if inf == nil {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		c.JSON(200, reporting.GetInfluencerBreakdown(infId, s.auth, s.Cfg, days, inf.Rep, inf.CurrentRep, c.Param("cid"), ""))
	}
}

func getAgencyInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		infId := c.Param("infId")
		inf := s.auth.GetInfluencer(infId)
		if inf == nil {
			c.JSON(500, misc.StatusErr("Error retrieving influencer!"))
			return
		}
		c.JSON(200, reporting.GetInfluencerBreakdown(infId, s.auth, s.Cfg, days, inf.Rep, inf.CurrentRep, "", c.Param("id")))
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

		influencers := getAllInfluencers(s)
		for _, infId := range influencers {
			inf := s.auth.GetInfluencer(infId)
			if inf == nil {
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
	Id string `json:"id,omitempty"`

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
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
				if inf := auth.GetInfluencer(u); inf != nil {
					if inf.RequestedCheck > 0 {
						tmpGreedy := &GreedyInfluencer{
							Id:             inf.Id,
							Address:        inf.Address,
							PendingPayout:  inf.PendingPayout,
							RequestedCheck: inf.RequestedCheck,
							CompletedDeals: inf.GetPostURLs(inf.LastCheck),
							Followers:      inf.GetFollowers(),
						}
						influencers = append(influencers, tmpGreedy)
					}
				}
				return nil
			})
		})
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

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			user := s.auth.GetUserTx(tx, infId)

			inf := auth.GetInfluencer(user)
			if inf == nil || inf.RequestedCheck == 0 {
				return auth.ErrInvalidID
			}

			if inf.PendingPayout == 0 {
				return ErrPayout
			}

			check, err := lob.CreateCheck(inf.Name, inf.Address, inf.PendingPayout, s.Cfg.Sandbox)
			if err != nil {
				return err
			}

			inf.Payouts = append(inf.Payouts, check)
			inf.PendingPayout = 0
			inf.RequestedCheck = 0
			inf.LastCheck = int32(time.Now().Unix())

			// Save the influencer
			return user.StoreWithData(s.auth, tx, inf)
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

		var inf *auth.Influencer
		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			user := s.auth.GetUserTx(tx, infId)

			inf = auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		for _, d := range inf.ActiveDeals {
			if d.CampaignId == cid && d.Perk != nil {
				d.Perk.Status = true
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

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			user := s.auth.GetUserTx(tx, infId)

			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			if inf.Banned {
				return ErrSorry
			}

			if inf.PendingPayout < 50 {
				return ErrInvalidFunds
			}

			if inf.LastCheck > 0 && inf.LastCheck > now-THIRTY_DAYS {
				return ErrThirtyDays
			}

			if inf.Address == nil {
				return ErrAddress
			}

			if c.Query("skipTax") != "1" && !inf.HasSigned {
				return ErrTax
			}

			inf.RequestedCheck = int32(time.Now().Unix())

			// Save the influencer
			return user.StoreWithData(s.auth, tx, inf)
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

		var (
			inf  *auth.Influencer
			user = auth.GetCtxUser(c)
			err  error
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}

			if inf = auth.GetInfluencer(user); inf == nil {
				return auth.ErrInvalidID
			}
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
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

		now := int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			user := s.auth.GetUserTx(tx, infId)

			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}

			if inf.Address == nil {
				return ErrAddress
			}

			var isAmerican bool
			if strings.ToLower(inf.Address.Country) == "us" {
				isAmerican = true
			}

			sigId, err := hellosign.SendSignatureRequest(inf.Name, user.Email, inf.Id, isAmerican, s.Cfg.Sandbox)
			if err != nil {
				return err
			}

			inf.SignatureId = sigId
			inf.RequestedTax = now

			// Save the influencer
			return user.StoreWithData(s.auth, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		// Insert log
		c.JSON(200, misc.StatusOK(infId))
	}
}

// Scraps

func postScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			sc  influencer.Scrap
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&sc); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if len(sc.TwitterID)+len(sc.FacebookID)+len(sc.InstagramID)+len(sc.YouTubeID) == 0 {
			c.JSON(400, misc.StatusErr("Scrap must have atleast one social network"))
			return
		}

		if len(sc.TwitterID) > 0 {
			sc.TwitterURL = "https://twitter.com/" + sc.TwitterID
		}

		if len(sc.FacebookID) > 0 {
			sc.FacebookURL = "https://www.facebook.com/" + sc.FacebookID
		}

		if len(sc.InstagramID) > 0 {
			sc.InstagramURL = "https://www.instagram.com/" + sc.InstagramID
		}

		if len(sc.YouTubeID) > 0 {
			sc.YouTubeURL = "https://www.youtube.com/user/" + sc.YouTubeID
		}

		sc.EmailAddress = misc.TrimEmail(sc.EmailAddress)

		// Save the Scrap
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if sc.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Scrap); err != nil {
				return
			}

			var (
				b []byte
			)

			if b, err = json.Marshal(sc); err != nil {
				return err
			}

			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Scrap, sc.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(sc.Id))
	}
}

func putScrap(s *Server) gin.HandlerFunc {
	// Updates info for the scrap (called via admin dash)
	return func(c *gin.Context) {
		var (
			b   []byte
			upd influencer.Scrap
			err error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if len(upd.Categories) == 0 {
			c.JSON(400, misc.StatusErr("Please pass valid categories"))
			return
		}
		upd.Categories = common.LowerSlice(upd.Categories)
		for _, cat := range upd.Categories {
			if _, ok := common.CATEGORIES[cat]; !ok {
				c.JSON(400, "Invalid category!")
				return
			}
		}

		if upd.Gender != "m" && upd.Gender != "f" {
			c.JSON(400, misc.StatusErr("Please pass a valid gender!"))
			return
		}

		// If a geo is passed and it doesnt have state + country..
		if upd.Geo != nil && upd.Geo.State == "" && upd.Geo.Country == "" {
			c.JSON(400, misc.StatusErr("Please pass a valid geo!"))
			return
		}

		var (
			scrapId = c.Param("id")
			scrap   influencer.Scrap
		)

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).Get([]byte(scrapId))
			return nil
		})

		if err = json.Unmarshal(b, &scrap); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling scrap"))
			return
		}

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			scrap.Geo = upd.Geo
			scrap.Gender = upd.Gender
			scrap.Categories = upd.Categories
			if b, err = json.Marshal(scrap); err != nil {
				return err
			}

			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Scrap, scrap.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(scrapId))
	}
}

func getScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			scrapId = c.Param("id")
			scrap   influencer.Scrap
			b       []byte
			err     error
		)

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).Get([]byte(scrapId))
			return nil
		})

		if err = json.Unmarshal(b, &scrap); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling scrap"))
			return
		}

		c.JSON(200, scrap)
	}
}

var ErrLimit = errors.New("Reached scrap limit!")

func getIncompleteScraps(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var scraps []*influencer.Scrap
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
				var sc influencer.Scrap
				if err := json.Unmarshal(v, &sc); err != nil {
					log.Println("error when unmarshalling scrap", string(v))
					return nil
				}
				if sc.Geo == nil && sc.Gender == "" && len(sc.Categories) == 0 {
					scraps = append(scraps, &sc)
				}

				if len(scraps) >= 10 {
					return ErrLimit
				}
				return
			})
			return nil
		})
		c.JSON(200, scraps)
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
		filename, err := saveImageToDisk(s.Cfg.ImagesDir+bucket+"/"+id, upd.Data, bucket, id)
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
		var (
			infId = c.Param("influencerId")
			user  = auth.GetCtxUser(c)
			rec   *geo.GeoRecord
		)

		if err := s.db.View(func(tx *bolt.Tx) (err error) {
			if infId != user.ID {
				user = s.auth.GetUserTx(tx, infId)
			}
			inf := auth.GetInfluencer(user)
			if inf == nil {
				return auth.ErrInvalidID
			}
			rec = inf.GetLatestGeo()
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, rec)
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

		inf := s.auth.GetInfluencer(infId)
		if inf == nil {
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
			log.Println("ERROR SAVING CLICK!", err)
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
