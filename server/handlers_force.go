package server

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
)

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

		var (
			fApp ForceApproval
			err  error
		)

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

		store, _ := budget.GetBudgetInfo(s.db, s.Cfg, campaignId, "")
		if store.IsClosed(&cmp) {
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
