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
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
)

///////// Talent Agencies ///////////
func putTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ag  common.TalentAgency
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&ag); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if ag.UserId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid user ID"))
			return
		}

		if ag.Fee == 0 || ag.Fee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid agency fee"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			// Insert a check for whether the user id exists in the "User" bucket here

			if ag.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.TalentAgency); err != nil {
				return
			}
			if b, err = json.Marshal(ag); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.TalentAgency, ag.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(ag.Id))
	}
}

func updateTalentAgency(s *Server) gin.HandlerFunc {
	// Overrwrites any of the agency attributes
	// NOTE: Must send full struct filled out with previous returned values
	return func(c *gin.Context) {
		var (
			ag  common.TalentAgency
			err error
			b   []byte
		)
		id := c.Params.ByName("id")
		if id == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid agency ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(id))
			return nil
		})

		if err = json.Unmarshal(b, &ag); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling agency"))
			return
		}

		var upd common.TalentAgency
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if ag.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}
		ag.Name = upd.Name

		if upd.Fee == 0 || upd.Fee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid agency fee"))
			return
		}
		// NOTE: Fee changes will only reflect for new campaigns AND old campaigns
		// AFTER the first of the month (when billing runs!)
		ag.Fee = upd.Fee

		// Save the Agency
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if b, err = json.Marshal(ag); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.TalentAgency, ag.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(ag.Id))
	}
}

func getTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			ag  common.TalentAgency
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(c.Params.ByName("id")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &ag); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, ag)
	}
}

func getAllTalentAgencies(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agenciesAll := make([]*common.TalentAgency, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).ForEach(func(k, v []byte) (err error) {
				ag := &common.TalentAgency{}
				if err := json.Unmarshal(v, ag); err != nil {
					log.Println("error when unmarshalling agency", string(v))
					return nil
				}
				agenciesAll = append(agenciesAll, ag)
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, agenciesAll)
	}
}

func delTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var ag *common.TalentAgency

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(agId)), &ag)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.TalentAgency, agId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(agId))
	}
}

///////// Ad Agencies /////////
func putAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ag  common.AdAgency
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&ag); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if ag.UserId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid user ID"))
			return
		}

		if ag.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}

		if ag.Fee == 0 || ag.Fee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid agency fee"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			// Insert a check for whether the user id exists in the "User" bucket here

			if ag.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.AdAgency); err != nil {
				return
			}
			if b, err = json.Marshal(ag); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.AdAgency, ag.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(ag.Id))
	}
}

func updateAdAgency(s *Server) gin.HandlerFunc {
	// Overrwrites any of the ad agency attributes
	// NOTE: Must send full struct filled out with previous returned values
	return func(c *gin.Context) {
		var (
			ag  common.AdAgency
			err error
			b   []byte
		)
		id := c.Params.ByName("id")
		if id == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid agency ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.AdAgency)).Get([]byte(id))
			return nil
		})

		if err = json.Unmarshal(b, &ag); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling agency"))
			return
		}

		var upd common.AdAgency
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if ag.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}
		ag.Name = upd.Name

		if upd.Fee == 0 || upd.Fee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid agency fee"))
			return
		}
		// NOTE: Fee changes will only reflect for new campaigns AND old campaigns
		// AFTER the first of the month (when billing runs!)
		ag.Fee = upd.Fee

		// Save the Agency
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if b, err = json.Marshal(ag); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.AdAgency, ag.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(ag.Id))
	}
}

func getAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			ag  common.AdAgency
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.AdAgency)).Get([]byte(c.Params.ByName("id")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &ag); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, ag)
	}
}

func getAllAdAgencies(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agenciesAll := make([]*common.AdAgency, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.AdAgency)).ForEach(func(k, v []byte) (err error) {
				ag := &common.AdAgency{}
				if err := json.Unmarshal(v, ag); err != nil {
					log.Println("error when unmarshalling agency", string(v))
					return nil
				}
				agenciesAll = append(agenciesAll, ag)
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, agenciesAll)
	}
}

func delAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var ag *common.AdAgency

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.AdAgency)).Get([]byte(agId)), &ag)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.AdAgency, agId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(agId))
	}
}

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			adv common.Advertiser
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&adv); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if adv.AgencyId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid agency ID"))
			return
		}

		if adv.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}

		if adv.ExchangeFee == 0 || adv.ExchangeFee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid exchange fee"))
			return
		}

		if adv.DspFee == 0 || adv.DspFee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid DSP fee"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if adv.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Advertiser); err != nil {
				return
			}

			if b, err = json.Marshal(adv); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Advertiser, adv.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(adv.Id))
	}
}

func updateAdvertiser(s *Server) gin.HandlerFunc {
	// Overrwrites any of the advertiser attributes
	return func(c *gin.Context) {
		var (
			adv common.Advertiser
			err error
			b   []byte
		)
		id := c.Params.ByName("id")
		if id == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid advertiser ID"))
			return
		}

		s.db.View(func(tx *bolt.Tx) error {
			b = tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(id))
			return nil
		})

		if err = json.Unmarshal(b, &adv); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling agency"))
			return
		}

		var upd common.Advertiser
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if upd.AgencyId == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid agency ID"))
			return
		}
		adv.AgencyId = upd.AgencyId

		if upd.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid name"))
			return
		}
		adv.Name = upd.Name

		if upd.ExchangeFee == 0 || upd.ExchangeFee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid exchange fee"))
			return
		}
		adv.ExchangeFee = upd.ExchangeFee

		if upd.DspFee == 0 || upd.DspFee > 0.99 {
			c.JSON(400, misc.StatusErr("Please provide a valid DSP fee"))
			return
		}
		adv.DspFee = upd.DspFee
		// NOTE: Fee changes will only reflect for new campaigns OR old campaigns
		// AFTER the first of the month (when billing runs!)

		// Save the Advertiser
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if b, err = json.Marshal(adv); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Advertiser, adv.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(adv.Id))
	}
}

func getAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			g   common.Advertiser
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(c.Params.ByName("id")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g)
	}
}

func getAdvertisersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAgency := c.Params.ByName("id")
		advertisers := make([]*common.Advertiser, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).ForEach(func(k, v []byte) (err error) {
				adv := &common.Advertiser{}
				if err := json.Unmarshal(v, adv); err != nil {
					log.Println("error when unmarshalling advertiser", string(v))
					return nil
				}
				if adv.AgencyId == targetAgency {
					advertisers = append(advertisers, adv)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		c.JSON(200, advertisers)
	}
}

func delAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		gId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Advertiser
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Advertiser, gId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(gId))
	}
}

///////// Campaigns /////////

func putCampaign(s *Server) gin.HandlerFunc {
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

		// Sanitize methods
		sanitized := []string{}
		for _, ht := range cmp.Tags {
			sanitized = append(sanitized, sanitizeHash(ht))
		}
		cmp.Tags = sanitized
		cmp.Mention = sanitizeMention(cmp.Mention)
		cmp.Categories = lowerArr(cmp.Categories)
		if cmp.Whitelist != nil {
			cmp.Whitelist.Sanitize()
		}
		if cmp.Blacklist != nil {
			cmp.Blacklist.Sanitize()
		}

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
		cmp := common.GetCampaign(c.Params.ByName("id"), s.db, s.Cfg)
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
		targetAdv := c.Params.ByName("id")
		campaigns := make([]*common.Campaign, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
					// No need to display massive deal set
					cmp.Deals = nil
					campaigns = append(campaigns, cmp)
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

func getCampaignAssignedDeals(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			cmp common.Campaign
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(c.Params.ByName("campaignId")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &cmp); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		deals := make([]*common.Deal, 0, 512)
		for _, d := range cmp.Deals {
			if d.Assigned > 0 && d.InfluencerId != "" && d.Completed == 0 {
				deals = append(deals, d)
			}
		}

		c.JSON(200, deals)
	}
}

func getCampaignCompletedDeals(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			cmp common.Campaign
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(c.Params.ByName("campaignId")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &cmp); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		deals := make([]*common.Deal, 0, 512)
		for _, d := range cmp.Deals {
			if d.Completed > 0 && d.InfluencerId != "" {
				deals = append(deals, d)
			}
		}

		c.JSON(200, deals)
	}
}

func delCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Campaign
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(id)), &g)
			if err != nil {
				return
			}

			g.Active = false

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
	Geos       []*misc.GeoRecord  `json:"geos,omitempty"`
	Categories []string           `json:"categories,omitempty"`
	Active     bool               `json:"active,omitempty"`
	Budget     float32            `json:"budget,omitempty"`
	Gender     string             `json:"gender,omitempty"` // "m" or "f" or "mf"
	Whitelist  *common.TargetList `json:"whitelist,omitempty"`
	Blacklist  *common.TargetList `json:"blacklist,omitempty"`
}

func updateCampaign(s *Server) gin.HandlerFunc {
	// Overrwrites any of the above campaign attributes
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			err error
			b   []byte
		)
		cId := c.Params.ByName("campaignId")
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

		cmp.Active = upd.Active

		cmp.Geos = upd.Geos
		cmp.Gender = upd.Gender
		cmp.Categories = lowerArr(upd.Categories)
		if upd.Whitelist != nil {
			cmp.Whitelist = upd.Whitelist.Sanitize()
		}
		if upd.Blacklist != nil {
			cmp.Blacklist = upd.Blacklist.Sanitize()
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

func putInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			load influencer.InfluencerLoad
			b    []byte
			err  error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&load); err != nil {
			log.Println("err", err)
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if load.Gender != "m" && load.Gender != "f" && load.Gender != "unicorn" {
			c.JSON(400, misc.StatusErr(ErrBadGender.Error()))
			return
		}

		if load.AgencyId == "" {
			c.JSON(400, misc.StatusErr(ErrNoAgency.Error()))
			return
		}

		if load.Name == "" {
			c.JSON(400, misc.StatusErr(ErrNoName.Error()))
			return
		}

		if load.Geo == nil {
			c.JSON(400, misc.StatusErr(ErrNoGeo.Error()))
			return
		}

		load.Category = strings.ToLower(load.Category)
		if _, ok := common.CATEGORIES[load.Category]; !ok {
			c.JSON(400, misc.StatusErr(ErrBadCat.Error()))
			return
		}

		inf, err := influencer.New(
			load.Name,
			load.TwitterId,
			load.InstagramId,
			load.FbId,
			load.YouTubeId,
			load.Gender,
			load.AgencyId,
			load.Category,
			load.Geo,
			s.Cfg)

		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if inf.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Influencer); err != nil {
				return
			}

			if b, err = json.Marshal(inf); err != nil {
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

func getInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, err := getInfluencerFromId(s, c.Params.ByName("id"))
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, inf)
	}
}

func delInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *influencer.Influencer
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Influencer, id)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}

func getInfluencersByCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetCat := c.Params.ByName("category")
		influencers := make([]*influencer.Influencer, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
				inf := &influencer.Influencer{}
				if err := json.Unmarshal(v, inf); err != nil {
					log.Println("error when unmarshalling influencer", string(v))
					return nil
				}
				if inf.Category == targetCat {
					influencers = append(influencers, inf)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, influencers)
	}
}

// func setFloor(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		raw := c.Params.ByName("floor")
// 		floor, err := strconv.ParseFloat(raw, 64)
// 		if err != nil {
// 			log.Println("ERR", err)
// 			c.JSON(500, misc.StatusErr("Invalid floor price"))
// 			return
// 		}

// 		// Alter influencer bucket
// 		var (
// 			inf influencer.Influencer
// 		)

// 		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
// 			b := tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))

// 			if err = json.Unmarshal(b, &inf); err != nil {
// 				return ErrUnmarshal
// 			}

// 			inf.FloorPrice = float32(floor)

// 			if b, err = json.Marshal(&inf); err != nil {
// 				return
// 			}

// 			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
// 				return
// 			}
// 			return
// 		}); err != nil {
// 			c.JSON(500, misc.StatusErr(err.Error()))
// 			return
// 		}

// 		c.JSON(200, misc.StatusOK(inf.Id))
// 	}
// }

func getInfluencersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAg := c.Params.ByName("agencyId")
		influencers := make([]*influencer.Influencer, 0, 512)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
				inf := &influencer.Influencer{}
				if err := json.Unmarshal(v, inf); err != nil {
					log.Println("error when unmarshalling influencer", string(v))
					return nil
				}
				if inf.AgencyId == targetAg {
					influencers = append(influencers, inf)
				}
				return
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
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
			err error
			inf influencer.Influencer
		)

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			b := tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))

			id := c.Params.ByName("id")
			if err = json.Unmarshal(b, &inf); err != nil {
				return ErrUnmarshal
			}

			switch c.Params.ByName("platform") {
			case "instagram":
				if err = inf.NewInsta(id, s.Cfg); err != nil {
					return
				}
			case "facebook":
				if err = inf.NewFb(id, s.Cfg); err != nil {
					return
				}
			case "twitter":
				if err = inf.NewTwitter(id, s.Cfg); err != nil {
					return
				}
			case "youtube":
				if err = inf.NewYouTube(id, s.Cfg); err != nil {
					return
				}
			default:
				return ErrPlatform
			}

			if b, err = json.Marshal(&inf); err != nil {
				return
			}

			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
				return
			}
			return
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

func setCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cat := strings.ToLower(c.Params.ByName("category"))
		if _, ok := common.CATEGORIES[cat]; !ok {
			c.JSON(400, misc.StatusErr(ErrBadCat.Error()))
			return
		}

		// Alter influencer bucket
		var (
			err error
			inf influencer.Influencer
		)

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			b := tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))

			if err = json.Unmarshal(b, &inf); err != nil {
				return ErrUnmarshal
			}

			inf.Category = cat

			if b, err = json.Marshal(&inf); err != nil {
				return
			}

			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
				return
			}
			return
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
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
		influencerId := c.Params.ByName("influencerId")

		var (
			lat, long float64
			err       error
		)

		rLat, err := strconv.ParseFloat(c.Params.ByName("lat"), 64)
		if err == nil {
			lat = rLat
		}
		rLong, err := strconv.ParseFloat(c.Params.ByName("long"), 64)
		if err == nil {
			long = rLong
		}

		if len(influencerId) == 0 {
			c.JSON(500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		deals := influencer.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, influencerId, "", misc.GetGeoFromCoords(lat, long, time.Now().Unix()), false, s.Cfg)
		c.JSON(200, deals)
	}
}

func assignDeal(s *Server) gin.HandlerFunc {
	// Influencer accepting deal
	// Must pass in influencer ID and deal ID
	return func(c *gin.Context) {
		dealId := c.Params.ByName("dealId")
		influencerId := c.Params.ByName("influencerId")
		campaignId := c.Params.ByName("campaignId")
		mediaPlatform := c.Params.ByName("platform")
		if _, ok := platform.ALL_PLATFORMS[mediaPlatform]; !ok {
			c.JSON(200, misc.StatusErr("This platform was not found"))
			return
		}

		// Lets quickly make sure that this deal is still available
		// via our GetAvailableDeals func
		var found bool
		foundDeal := &common.Deal{}
		currentDeals := influencer.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, influencerId, dealId, nil, true, s.Cfg)
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

			if !cmp.Active {
				return errors.New("Campaign is no longer active")
			}

			foundDeal.InfluencerId = influencerId
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
			var inf *influencer.Influencer
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(influencerId)), &inf)
			if err != nil {
				return err
			}

			if inf.ActiveDeals == nil || len(inf.ActiveDeals) == 0 {
				inf.ActiveDeals = []*common.Deal{}
			}
			inf.ActiveDeals = append(inf.ActiveDeals, foundDeal)

			// Save the Influencer
			if err = saveInfluencer(tx, inf, s.Cfg); err != nil {
				return err
			}

			// Save the campaign
			if err = saveCampaign(tx, cmp, s); err != nil {
				return err
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
			err error
			v   []byte
			g   influencer.Influencer
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g.ActiveDeals)
	}
}

func unassignDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		dealId := c.Params.ByName("dealId")
		influencerId := c.Params.ByName("influencerId")
		campaignId := c.Params.ByName("campaignId")

		if err := clearDeal(s, dealId, influencerId, campaignId, false); err != nil {
			c.JSON(200, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(dealId))
	}
}

func getDealsCompletedByInfluencer(s *Server) gin.HandlerFunc {
	// Get all deals completed by the influencer in the last X hours
	return func(c *gin.Context) {
		var (
			err error
			v   []byte
			g   influencer.Influencer
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		offset := c.Params.ByName("influencerId")
		if offset == "-1" {
			c.JSON(200, g.CompletedDeals)
		} else {
			offsetHours, err := strconv.Atoi(offset)
			if err != nil {
				c.JSON(400, misc.StatusErr("Please provide a valid gender target (m, f or mf)"))
				return
			}
			minTs := int32(time.Now().Unix()) - (60 * 60 * int32(offsetHours))
			deals := []*common.Deal{}
			for _, d := range g.CompletedDeals {
				if d.Completed >= minTs {
					deals = append(deals, d)
				}
			}
			c.JSON(200, deals)
		}
	}
}

// Budget
func getBudgetInfo(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, err := budget.GetBudgetInfo(s.budgetDb, s.Cfg, c.Params.ByName("id"), "")
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
func getStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := reporting.GetStatsByCampaign(c.Params.ByName("cid"), s.reportingDb, s.Cfg)
		if err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}
		c.JSON(200, stats)
	}
}

func getCampaignReport(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.Params.ByName("cid")
		if cid == "" {
			c.JSON(500, misc.StatusErr("Please pass in a valid campaign ID"))
			return
		}

		from := reporting.GetReportDate(c.Params.ByName("from"))
		to := reporting.GetReportDate(c.Params.ByName("to"))
		if from.IsZero() || to.IsZero() || to.Before(from) {
			c.JSON(500, misc.StatusErr("Invalid date range!"))
			return
		}

		if err := reporting.GenerateCampaignReport(c.Writer, s.db, s.reportingDb, cid, from, to, s.Cfg); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
		}
	}
}
