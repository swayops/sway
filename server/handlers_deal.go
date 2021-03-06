package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
)

///////// Deals /////////
func getDealsForCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cmp := common.GetCampaign(c.Param("id"), s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(fmt.Sprintf("Failed for campaign")))
			return
		}

		misc.WriteJSON(c, 200, getDealsForCmp(s, cmp, false))
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

		scraps := s.Scraps.GetStore()
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

		misc.WriteJSON(c, 200, matches)
	}
}

func getKeywords(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		matches := make(map[string]int)
		includeImagga := c.Query("imagga") == "t"
		for _, inf := range s.auth.Influencers.GetAll() {
			if includeImagga {
				for _, kw := range inf.Keywords {
					matches[kw+"-imagga"] += 1
				}
			}

			if inf.Instagram != nil && inf.Instagram.Bio != "" {
				for _, kw := range strings.Split(inf.Instagram.Bio, " ") {
					if len(kw) > 3 {
						matches[kw] += 1
					}
				}
			}
		}

		scraps := s.Scraps.GetStore()
		for _, sc := range scraps {
			if includeImagga {
				for _, kw := range sc.Keywords {
					matches[kw+"-imagga"] += 1
				}
			}

			if sc.InstaData != nil && sc.InstaData.Bio != "" {
				for _, kw := range strings.Split(sc.InstaData.Bio, " ") {
					if len(kw) > 3 {
						matches[kw] += 1
					}
				}
			}
		}

		type kv struct {
			Key   string
			Value int
		}

		var ss []kv
		for k, v := range matches {
			if v > 100 {
				ss = append(ss, kv{k, v})
			}
		}

		sort.Slice(ss, func(i, j int) bool {
			return ss[i].Value > ss[j].Value
		})

		misc.WriteJSON(c, 200, ss)
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
			misc.WriteJSON(c, 500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		deals, _ := inf.GetAvailableDeals(s.Campaigns, s.Audiences, s.db, "", "",
			geo.GetGeoFromCoords(lat, long, int32(time.Now().Unix())), false, s.getTalentAgencyFee(inf.AgencyId), s.Cfg)
		misc.WriteJSON(c, 200, deals)
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
			misc.WriteJSON(c, 500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		if len(dealId) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Deal ID undefined"))
			return
		}

		if len(campaignId) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Campaign ID undefined"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		deals, _ := inf.GetAvailableDeals(s.Campaigns, s.Audiences, s.db, campaignId, dealId, nil, true, s.getTalentAgencyFee(inf.AgencyId), s.Cfg)
		if len(deals) != 1 {
			misc.WriteJSON(c, 500, misc.StatusErr("Deal no longer available"))
			return
		}
		misc.WriteJSON(c, 200, deals[0])
	}
}

type Post struct {
	Image   string `json:"img,omitempty"`
	Message string `json:"caption,omitempty"`
}

func submitPost(s *Server) gin.HandlerFunc {
	// Influencer submitting posts for approval
	return func(c *gin.Context) {
		var (
			campaignId = c.Param("campaignId")
			infId      = c.Param("influencerId")
		)

		if len(infId) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		if len(campaignId) == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Campaign ID undefined"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		var found *common.Deal
		for _, deal := range inf.ActiveDeals {
			if deal.CampaignId == campaignId {
				found = deal
				break
			}
		}

		if found == nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Deal not found"))
			return
		}

		// Does this influencer even need to submit a submission?
		user := s.auth.GetUser(found.AdvertiserId)
		if user == nil || user.Advertiser == nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Advertiser not found"))
			return
		}

		cmp, ok := s.Campaigns.Get(found.CampaignId)
		if !ok {
			misc.WriteJSON(c, 400, misc.StatusErr("Campaign undefined"))
			return
		}

		if !cmp.RequiresSubmission {
			misc.WriteJSON(c, 400, misc.StatusErr("Deal does not require prior submission. You are free to post!"))
			return
		}

		var (
			sub common.Submission
			err error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&sub); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		if len(sub.ImageData) == 0 && len(sub.ContentURL) == 0 {
			// No media content sent!
			misc.WriteJSON(c, 400, misc.StatusErr("No media content!"))
			return
		}

		if len(sub.Message) == 0 {
			misc.WriteJSON(c, 400, misc.StatusErr("No message content!"))
			return
		}

		sub.SanitizeContent()

		if len(sub.ImageData) != 0 {
			for idx, imgData := range sub.ImageData {
				pre := strconv.Itoa(idx) + "-"
				if !strings.HasPrefix(imgData, "data:image/") {
					misc.AbortWithErr(c, 400, errors.New("Please provide a valid image"))
					return
				}
				filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Campaign, pre+found.Id), imgData, pre+found.Id, "", 750, 389)
				if err != nil {
					misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
					return
				}

				sub.ContentURL = append(sub.ContentURL, getImageUrl(s, s.Cfg.Bucket.Campaign, "dash", filename, false))
			}

			sub.ImageData = nil
		}

		found.Submission = &sub

		if err := saveAllActiveDeals(s, inf); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		// Email the advertiser
		if s.Cfg.ReplyMailClient() != nil && !s.Cfg.Sandbox {
			email := templates.NotifySubmissionEmail.Render(map[string]interface{}{"Name": user.Name, "InfluencerName": found.InfluencerName, "CampaignName": found.CampaignName})
			emailAdvertiser(s, user, email, "A submitted post by "+found.InfluencerName+" is awaiting your approval")
		}

		s.Notify("Deal post submitted!", fmt.Sprintf("Influencer %s has submitted post for %s", infId, campaignId))

		misc.WriteJSON(c, 200, found)
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
			misc.WriteJSON(c, 500, misc.StatusErr("This platform was not found"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
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

		currentDeals, _ := inf.GetAvailableDeals(s.Campaigns, s.Audiences, s.db, campaignId, dealId, nil, false, s.getTalentAgencyFee(inf.AgencyId), s.Cfg)
		for _, deal := range currentDeals {
			if deal.CampaignId == campaignId && deal.Assigned == 0 && deal.InfluencerId == "" {
				if dbg || deal.Id == dealId {
					found = true
					foundDeal = deal
				}
			}
		}

		if !found {
			misc.WriteJSON(c, 500, misc.StatusErr("Unforunately, the requested deal is no longer available!"))
			return
		}

		// Assign the deal & Save the Campaign
		// DEALS are located in the INFLUENCER struct AND the CAMPAIGN struct
		var cmp *common.Campaign
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
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

				if cmp.Perks.Count == 0 && cmp.Monthly {
					// Lets email the advertiser letting them know there are no more
					// perks available if it's a monthly (recurring) campaign

					user := s.auth.GetUser(cmp.AdvertiserId)
					if user == nil || user.Advertiser == nil {
						misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid advertiser ID"))
						return
					}

					email := templates.NotifyEmptyPerkEmail.Render(map[string]interface{}{"ID": cmp.Id, "Campaign": cmp.Name, "Perk": cmp.Perks.Name, "Name": user.Advertiser.Name})
					emailAdvertiser(s, user, email, "You have no remaining perks for the campaign "+cmp.Name)
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		go func() {
			// Lets send them deal instructions if there are any!
			if cmp != nil && foundDeal != nil {
				s.Notify("Deal accepted!", fmt.Sprintf("%s just accepted a deal for %s", inf.Name, cmp.Name))
				assignDealEmail(s, cmp, foundDeal, &inf)
			}
		}()

		misc.WriteJSON(c, 200, foundDeal)
	}
}

func getDealsAssignedToInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		var deals []*common.Deal
		for _, d := range inf.ActiveDeals {
			deals = append(deals, d)
		}

		misc.WriteJSON(c, 200, deals)
	}
}

func unassignDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		dealId := c.Param("dealId")
		influencerId := c.Param("influencerId")
		campaignId := c.Param("campaignId")

		if err := clearDeal(s, dealId, influencerId, campaignId, false); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		// Lets email the influencer telling them the deal is OVAH!
		inf, ok := s.auth.Influencers.Get(influencerId)
		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)

		if ok && cmp != nil {
			if err := inf.DealUpdate(cmp, s.Cfg); err != nil {
				s.Alert("Failed to give influencer a deal update: "+inf.Id, err)
				misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
				return
			}
		}

		misc.WriteJSON(c, 200, misc.StatusOK(dealId))
	}
}

func sendInstructions(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		dealId := c.Param("dealId")

		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		cmp := common.GetCampaign(c.Param("campaignId"), s.db, s.Cfg)
		if ok && cmp != nil {
			if deal, ok := cmp.Deals[dealId]; ok && deal.IsActive() {
				assignDealEmail(s, cmp, deal, &inf)
			}
		}

		misc.WriteJSON(c, 200, misc.StatusOK(dealId))
	}
}

func getDealsCompletedByInfluencer(s *Server) gin.HandlerFunc {
	// Get all deals completed by the influencer in the last X hours
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, inf.CompletedDeals)
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

	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

func getAllActiveDeals(s *Server) gin.HandlerFunc {
	// Retrieves all active deals in the system
	return func(c *gin.Context) {
		var deals []*SimpleActive
		for _, cmp := range s.Campaigns.GetStore() {
			if !cmp.IsValid() {
				continue
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
						Email:        infClean.EmailAddress,
						Name:         infClean.Name,
					})
				}
			}
		}
		misc.WriteJSON(c, 200, deals)
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
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
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
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid deal!"))
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
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(inf.Id))
	}
}
