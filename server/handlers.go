package server

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
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
		var all []*auth.TalentAgency
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.TalentAgencyScope, func(u *auth.User) error {
				if ag := auth.GetTalentAgency(u); ag != nil {
					all = append(all, ag)
				}
				return nil
			})
		})
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
	return func(c *gin.Context) {
		var all []*auth.AdAgency
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.AdAgencyScope, func(u *auth.User) error {
				if ag := auth.GetAdAgency(u); ag != nil {
					all = append(all, ag)
				}
				return nil
			})
		})
		c.JSON(200, all)
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
func postCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
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

		if cmp.AdvertiserId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		if cmp.AgencyId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid agency ID"))
			return
		}

		if !cmp.Twitter && !cmp.Facebook && !cmp.Instagram && !cmp.YouTube {
			c.JSON(400, misc.StatusErr("Please target atleast one social network"))
			return
		}

		if len(cmp.Tags) == 0 && cmp.Mention == "" && cmp.Link == "" {
			c.JSON(400, misc.StatusErr("Please provide a required tag, mention or link"))
			return
		}

		for i, ht := range cmp.Tags {
			cmp.Tags[i] = sanitizeHash(ht)
		}
		cmp.Mention = sanitizeMention(cmp.Mention)
		cmp.Categories = common.LowerSlice(cmp.Categories)

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign); err != nil {
				return
			}
			addDealsToCampaign(&cmp)
			return saveCampaign(tx, &cmp, s)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Create their budget key
		dspFee, exchangeFee := getAdvertiserFees(s, cmp.AdvertiserId)
		if err = budget.CreateBudgetKey(s.budgetDb, s.Cfg, &cmp, 0, 0, dspFee, exchangeFee, false); err != nil {
			log.Println("Error creating budget key!", err)
			c.JSON(500, misc.StatusErr(err.Error()))
			return
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
	Geos       []*misc.GeoRecord `json:"geos,omitempty"`
	Categories []string          `json:"categories,omitempty"`
	Status     bool              `json:"status,omitempty"`
	Budget     float64           `json:"budget,omitempty"`
	Gender     string            `json:"gender,omitempty"` // "m" or "f" or "mf"
	Name       string            `json:"name,omitempty"`
}

func putCampaign(s *Server) gin.HandlerFunc {
	// Overrwrites any of the above campaign attributes
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
			b   []byte
		)
		cId := c.Param("campaignId")
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

		var upd CampaignUpdate
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if upd.Gender != "m" && upd.Gender != "f" && upd.Gender != "mf" {
			c.JSON(400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
			return
		}

		if upd.Budget == 0 {
			c.JSON(400, misc.StatusErr("Please provide a valid budget"))
			return
		}

		if cmp.Budget != upd.Budget {
			// Update their budget!
			dspFee, exchangeFee := getAdvertiserFees(s, cmp.AdvertiserId)
			if err = budget.AdjustBudget(s.budgetDb, s.Cfg, cmp.Id, upd.Budget, dspFee, exchangeFee); err != nil {
				log.Println("Error creating budget key!", err)
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
			cmp.Budget = upd.Budget
		}

		cmp.Status = upd.Status

		cmp.Geos = upd.Geos
		cmp.Gender = upd.Gender
		cmp.Categories = common.LowerSlice(upd.Categories)
		cmp.Name = upd.Name

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
		targetAg := c.Param("agencyId")
		var influencers []*auth.Influencer
		s.db.View(func(tx *bolt.Tx) error {
			return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
				inf := auth.GetInfluencer(u)
				if inf == nil {
					return nil
				}
				if inf.AgencyId == targetAg {
					inf.Clean()
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

func setGeo(s *Server) gin.HandlerFunc {
	// Sets the default geo for the influencer
	return func(c *gin.Context) {
		var (
			geo misc.GeoRecord
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&geo); err != nil || geo == nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
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

			inf.Geo = &geo

			// Save the influencer
			return user.StoreWithData(s.auth, tx, inf)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(infId))
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

		deals := inf.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, "",
			misc.GetGeoFromCoords(lat, long, time.Now().Unix()), false, s.Cfg)
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
			c.JSON(200, misc.StatusErr("This platform was not found"))
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
		var found bool
		foundDeal := &common.Deal{}
		currentDeals := inf.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, dealId, nil, true, s.Cfg)
		for _, deal := range currentDeals {
			if deal.Spendable > 0 && deal.Id == dealId && deal.CampaignId == campaignId && deal.Assigned == 0 && deal.InfluencerId == "" {
				found = true
				foundDeal = deal
			}
		}

		if !found {
			c.JSON(200, misc.StatusErr("Unforunately, the requested deal is no longer available!"))
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

			if !cmp.Status {
				return errors.New("Campaign is no longer active")
			}

			foundDeal.InfluencerId = infId
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
			cmp.Deals[dealId] = foundDeal

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
			c.JSON(200, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(dealId))
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
			c.JSON(200, misc.StatusErr(err.Error()))
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

// Reporting
func getRawStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := reporting.GetStatsByCampaign(c.Param("cid"), s.reportingDb, s.Cfg)
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, stats)
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

		if err := reporting.GenerateCampaignReport(c.Writer, s.db, s.reportingDb, cid, from, to, s.Cfg); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
		}
	}
}

func getCampaignStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		c.JSON(200, reporting.GetCampaignBreakdown(c.Param("cid"), s.reportingDb, s.Cfg, days))
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
		c.JSON(200, reporting.GetInfluencerBreakdown(infId, s.reportingDb, s.Cfg, days, inf.Rep, inf.CurrentRep, ""))
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

		c.JSON(200, reporting.GetInfluencerBreakdown(infId, s.reportingDb, s.Cfg, days, inf.Rep, inf.CurrentRep, c.Param("cid")))
	}
}
