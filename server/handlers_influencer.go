package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

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
			misc.WriteJSON(c, 500, misc.StatusErr("Please provide a valid influencer ID"))
			return
		}

		var (
			upd     InfluencerUpdate
			err     error
			isAdmin = auth.GetCtxUser(c).Admin
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		// Update platforms
		if upd.InstagramId != "" {
			if inf.Instagram == nil || (inf.Instagram != nil && upd.InstagramId != inf.Instagram.UserName) {
				// Make sure that the instagram id has actually been updated
				err = inf.NewInsta(upd.InstagramId, s.Scraps.GetKeywords(inf.EmailAddress, upd.InstagramId, s.Cfg.Sandbox), s.Cfg)
				if err != nil {
					misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
					misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
					misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
					return
				}
			}
		} else {
			inf.Twitter = nil
		}

		if upd.YouTubeId != "" {
			if inf.YouTube == nil || (inf.YouTube != nil && upd.YouTubeId != inf.YouTube.UserName) {
				// Make sure that the id has actually been updated
				err = inf.NewYouTube(upd.YouTubeId, s.Scraps.GetKeywords(inf.EmailAddress, upd.YouTubeId, s.Cfg.Sandbox), s.Cfg)
				if err != nil {
					misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
				misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
				return
			}

			if !geo.IsValidGeo(&geo.GeoRecord{State: cleanAddr.State, Country: cleanAddr.Country}) {
				misc.WriteJSON(c, 400, misc.StatusErr("Address does not convert to a valid geo!"))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		if upd.Name != nil {
			name := strings.TrimSpace(*upd.Name)
			if len(strings.Split(name, " ")) < 2 {
				misc.WriteJSON(c, 400, misc.StatusErr(ErrNoName.Error()))
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
			changed, err := savePassword(s, tx, upd.OldPass, upd.Pass, upd.Pass2, user, isAdmin)

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

		misc.WriteJSON(c, 200, misc.StatusOK(inf.Id))
	}
}

type AuditSet struct {
	Categories []string `json:"categories,omitempty"`
	Gender     string   `json:"gender,omitempty"`
	BrandSafe  string   `json:"brandSafe,omitempty"`
}

func setAudit(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Please provide a valid influencer ID"))
			return
		}

		var (
			upd AuditSet
			err error
		)
		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&upd); err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		var filteredCats []string
		for _, cat := range upd.Categories {
			if _, ok := common.CATEGORIES[cat]; !ok {
				misc.WriteJSON(c, 400, misc.StatusErr(ErrBadCat.Error()))
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

		if upd.BrandSafe != "" {
			inf.BrandSafe = strings.ToLower(upd.BrandSafe)
		}

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		go inf.EmailAudit(s.Cfg)

		misc.WriteJSON(c, 200, misc.StatusOK(inf.Id))
	}
}

func getInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		inf, ok := s.auth.Influencers.Get(c.Param("id"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, inf)
	}
}

func testInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		if c.Query("key") != "7d7e8c4486c8" || now.Month() != time.September {
			misc.WriteJSON(c, 401, misc.StatusErr("Unauthorized"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("id"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, inf)
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
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
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
		misc.WriteJSON(c, 200, bio)
	}
}

func getCompletedDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		dealId := c.Param("dealId")
		if dealId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid deal id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
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
			misc.WriteJSON(c, 500, misc.StatusErr("deal not found"))
			return
		}

		misc.WriteJSON(c, 200, d)
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
		misc.WriteJSON(c, 200, influencers)
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
		misc.WriteJSON(c, 200, influencers)
	}
}

func setBan(s *Server) gin.HandlerFunc {
	// Sets the banned value for the influencer id
	return func(c *gin.Context) {
		ban, err := strconv.ParseBool(c.Params.ByName("state"))
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please submit a valid ban state"))
			return
		}

		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.Banned = ban

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Influencer has been banned!", fmt.Sprintf("Influencer %s has been banned", infId))

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func setStrike(s *Server) gin.HandlerFunc {
	// Sets the banned value for the influencer id
	return func(c *gin.Context) {
		reasons := c.Params.ByName("reasons")
		if reasons == "" {
			misc.WriteJSON(c, 400, misc.StatusErr("Please submit a valid reason"))
			return
		}

		var (
			infId      = c.Param("influencerId")
			campaignId = c.Param("campaignId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if campaignId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid campaign ID"))
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
				misc.WriteJSON(c, 500, misc.StatusErr("Strike has already been recorded!"))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Strike given!", fmt.Sprintf("Influencer %s has been given a strike (and the post has been allowed) for campaign %s", infId, campaignId))

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func addKeyword(s *Server) gin.HandlerFunc {
	// Manually add kw
	return func(c *gin.Context) {
		kw := c.Param("kw")
		if kw == "" {
			misc.WriteJSON(c, 400, misc.StatusErr("Please submit a valid keyword"))
			return
		}

		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.Keywords = append(inf.Keywords, kw)

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func skipGeo(s *Server) gin.HandlerFunc {
	// Manually add kw
	return func(c *gin.Context) {
		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		cid := c.Params.ByName("campaignId")
		if cid != "" && !misc.Contains(inf.GeoSkips, cid) {
			inf.GeoSkips = append(inf.GeoSkips, cid)

			if err := s.db.Update(func(tx *bolt.Tx) (err error) {
				return saveInfluencer(s, tx, inf)
			}); err != nil {
				misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
				return
			}
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func skipYield(s *Server) gin.HandlerFunc {
	// Skip max yield
	return func(c *gin.Context) {
		var (
			infId = c.Param("influencerId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		cid := c.Params.ByName("campaignId")
		if cid != "" && !misc.Contains(inf.GeoSkips, cid) {
			inf.SkipYield = append(inf.SkipYield, cid)

			if err := s.db.Update(func(tx *bolt.Tx) (err error) {
				return saveInfluencer(s, tx, inf)
			}); err != nil {
				misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
				return
			}
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

// func setSignature(s *Server) gin.HandlerFunc {
// 	// Manually set sig id
// 	return func(c *gin.Context) {
// 		sigId := c.Param("sigId")
// 		if sigId == "" {
// 			misc.WriteJSON(c, 400, misc.StatusErr("Please submit a valid sigId"))
// 			return
// 		}

// 		var (
// 			infId = c.Param("influencerId")
// 		)

// 		inf, ok := s.auth.Influencers.Get(infId)
// 		if !ok {
// 			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
// 			return
// 		}

// 		inf.SignatureId = sigId
// 		inf.RequestedCheck = int32(time.Now().Unix())

// 		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
// 			return saveInfluencer(s, tx, inf)
// 		}); err != nil {
// 			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
// 			return
// 		}

// 		misc.WriteJSON(c, 200, misc.StatusOK(infId))
// 	}
// }

func addDealCount(s *Server) gin.HandlerFunc {
	// Manually add a certain number of deals
	return func(c *gin.Context) {
		count, err := strconv.Atoi(c.Param("count"))
		if err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling campaign"))
			return
		}

		if cmp.Perks != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Cannot add deals to perk campaign"))
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

		misc.WriteJSON(c, 200, misc.StatusOK(""))
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
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body:"+err.Error()))
			return
		}

		if bonus.InfluencerID == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		if bonus.CampaignID == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(bonus.InfluencerID)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		parsed, err := url.Parse(bonus.PostURL)
		if err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid post URL"))
			return
		}

		bonus.PostURL = parsed.Host + parsed.Path
		if bonus.PostURL == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid post URL"))
			return
		}

		var foundDeal *common.Deal
		for _, d := range inf.CompletedDeals {
			if d.CampaignId == bonus.CampaignID {
				foundDeal = d
			}
		}

		if foundDeal == nil {
			misc.WriteJSON(c, 500, misc.StatusErr("deal not found"))
			return
		}

		// Force update saves all new posts and updates to recent data
		err = inf.ForceUpdate(s.Cfg)
		if err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("internal error with influencer update"))
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
			misc.WriteJSON(c, 500, misc.StatusErr("invalid post URL"))
			return
		}

		if err := saveAllCompletedDeals(s, inf); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(bonus.InfluencerID))
	}
}

func setFraud(s *Server) gin.HandlerFunc {
	// Sets the fraud check value for a deal
	return func(c *gin.Context) {
		fraud, err := strconv.ParseBool(c.Params.ByName("state"))
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please submit a valid fraud state"))
			return
		}

		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		cid := c.Param("campaignId")
		if cid == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		for _, d := range inf.ActiveDeals {
			if d.CampaignId == cid {
				d.SkipFraud = fraud
			}
		}

		if err := saveAllActiveDeals(s, inf); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Deal post allowed!", fmt.Sprintf("Deal for campaign %s and influencer %s has been allowed", cid, infId))

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
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
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		talentAgency := s.auth.GetTalentAgency(agId)
		if talentAgency == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(fmt.Sprintf("Could not find talent agency %s", inf.AgencyId)))
			return
		}

		inf.AgencyId = agId

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
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

			if (!inf.Male && !inf.Female) || len(inf.Categories) == 0 || inf.BrandSafe == "" {
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

		misc.WriteJSON(c, 200, influencers)
	}
}

func getCategories(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		misc.WriteJSON(c, 200, s.Categories)
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
					// SigID:          inf.SignatureId,
				}
				influencers = append(influencers, tmpGreedy)
			}
		}
		misc.WriteJSON(c, 200, influencers)
	}
}

var (
	ErrSorry        = errors.New("Sorry! You are currently not eligible for a check!")
	ErrInvalidFunds = errors.New("Must have atleast $10 USD to be paid out!")
	ErrFiveDays     = errors.New("Must wait atleast 5 days since last check to receive a payout!")
	ErrAddress      = errors.New("Please set an address for your profile!")
	ErrTax          = errors.New("Please fill out all necessary tax forms!")
)

const FIVE_DAYS = 60 * 60 * 24 * 5 // Five days in seconds

func requestCheck(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Delete the check and entry, send to lob
		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		now := int32(time.Now().Unix())

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if inf.IsBanned() {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrSorry.Error()))
			return
		}

		if inf.PendingPayout < 10 {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrInvalidFunds.Error()))
			return
		}

		if inf.LastCheck > 0 && inf.LastCheck > now-FIVE_DAYS {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrFiveDays.Error()))
			return
		}

		if inf.Address == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrAddress.Error()))
			return
		}

		// if c.Query("skipTax") != "1" && !inf.HasSigned {
		// 	misc.WriteJSON(c, 500, misc.StatusErr(ErrTax.Error()))
		// 	return
		// }

		inf.RequestedCheck = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		s.Notify("Check requested!", fmt.Sprintf("%s just requested a check of %f! Please check admin dash.", inf.Name, inf.PendingPayout))

		// Insert log
		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

// func emailTaxForm(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		// Delete the check and entry, send to lob
// 		infId := c.Param("influencerId")
// 		if infId == "" {
// 			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
// 			return
// 		}

// 		inf, ok := s.auth.Influencers.Get(infId)
// 		if !ok {
// 			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
// 			return
// 		}

// 		if inf.SignatureId != "" {
// 			misc.WriteJSON(c, 500, misc.StatusErr("Tax documents have already been sent! Please fill those out and allow us 4-8 hours to approve your information. Thank-you!"))
// 			return
// 		}

// 		if inf.Address == nil {
// 			misc.WriteJSON(c, 500, misc.StatusErr(ErrAddress.Error()))
// 			return
// 		}

// 		sigId, err := hellosign.SendSignatureRequest(inf.Name, inf.EmailAddress, inf.Id, inf.IsAmerican(), s.Cfg.Sandbox)
// 		if err != nil {
// 			s.Alert("Hellosign signature request failed for "+inf.Id, err)
// 			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
// 			return
// 		}

// 		inf.SignatureId = sigId
// 		inf.RequestedTax = int32(time.Now().Unix())

// 		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
// 			// Save the influencer
// 			return saveInfluencer(s, tx, inf)
// 		}); err != nil {
// 			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
// 			return
// 		}
// 		// Insert log
// 		misc.WriteJSON(c, 200, misc.StatusOK(infId))
// 	}
// }

func emptyPayout(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Empties out influencer's pending payout
		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		inf.PendingPayout = 0
		inf.RequestedCheck = 0
		inf.LastCheck = int32(time.Now().Unix())

		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func blockCampaign(s *Server) gin.HandlerFunc {
	// Blocks given campaign ID for given influencer
	return func(c *gin.Context) {
		var (
			infId      = c.Param("influencerId")
			campaignId = c.Param("campaignId")
		)

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if campaignId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid campaign ID"))
			return
		}

		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid campaign"))
			return
		}

		// Add the campaign ID to blacklist
		inf.Blacklist = append(inf.Blacklist, campaignId)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			// Save the influencer
			return saveInfluencer(s, tx, inf)
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
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

	// Maps to label "PERKS TO SHIP" on admin frontend
	return func(c *gin.Context) {
		var perks []PerkWithCmpInfo
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if !cmp.IsValid() {
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
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, perks)
	}
}

var ErrPayout = errors.New("Nothing to payout!")

func approveCheck(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Delete the check and entry, send to lob

		// Maps to label "CHECK PAYOUTS" on admin frontend
		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
			return
		}

		if inf.RequestedCheck == 0 || inf.PendingPayout == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrPayout.Error()))
			return
		}

		check, err := lob.CreateCheck(inf.Id, inf.Name, inf.Address, inf.PendingPayout, s.Cfg)
		if err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		if err := inf.CheckEmail(check, s.Cfg); err != nil {
			s.Alert("Failed to email check information to influencer "+inf.Id, err)
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

func approvePerk(s *Server) gin.HandlerFunc {
	// Maps to "SENT" button on Perks To Ship admin page
	return func(c *gin.Context) {
		infId := c.Param("influencerId")
		if infId == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid influencer id"))
			return
		}

		cid := c.Param("campaignId")
		if cid == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid campaign id"))
			return
		}

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr(auth.ErrInvalidID.Error()))
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
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(infId))
	}
}

type Inventory struct {
	ID        string `json:"id,omitempty"`
	Facebook  string `json:"facebook,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	Twitter   string `json:"twitter,omitempty"`
	YouTube   string `json:"youtube,omitempty"`

	Followers int64 `json:"followers,omitempty"`
}

func getInventoryByState(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Gets influencers and scraps that are in a particular state
		var targetGeo []*geo.GeoRecord
		targetGeo = append(targetGeo, &geo.GeoRecord{State: c.Param("state"), Country: "US"})

		var inv []*Inventory
		for _, inf := range s.auth.Influencers.GetAll() {
			if !geo.IsGeoMatch(targetGeo, inf.GetLatestGeo()) {
				continue
			}

			inf := inf.Clean()
			inv = append(inv, &Inventory{
				ID:        inf.Id,
				Facebook:  inf.FbUsername,
				Instagram: inf.InstaUsername,
				Twitter:   inf.TwitterUsername,
				YouTube:   inf.YTUsername,
				Followers: inf.GetFollowers(),
			},
			)
		}

		scraps := s.Scraps.GetStore()
		for _, sc := range scraps {
			if !geo.IsGeoMatch(targetGeo, sc.Geo) {
				continue
			}

			tmp := &Inventory{Followers: sc.Followers}
			if sc.Facebook {
				tmp.Facebook = sc.Name
			}

			if sc.Instagram {
				tmp.Instagram = sc.Name
			}

			if sc.YouTube {
				tmp.YouTube = sc.Name
			}

			if sc.Twitter {
				tmp.Twitter = sc.Name
			}

			inv = append(inv, tmp)
		}

		misc.WriteJSON(c, 200, inv)
	}
}

func getAllHandles(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		platform := c.Param("platform")
		if platform == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("invalid platform id"))
			return
		}

		// Could have a comma seperated list of IDs that are wanted
		rawIDs := strings.Split(c.Query("filter"), ",")
		ids := []string{}
		for _, val := range rawIDs {
			if val != "" {
				ids = append(ids, val)
			}
		}

		if c.Query("flw") != "" {
			out := make(map[string]int64)

			for _, inf := range s.auth.Influencers.GetAll() {
				switch platform {
				case "insta":
					if inf.Instagram != nil {
						out[inf.Instagram.UserName] = int64(inf.Instagram.Followers)
					}
				}
			}

			scraps := s.Scraps.GetStore()
			for _, sc := range scraps {
				switch platform {
				case "insta":
					if sc.InstaData != nil {
						out[sc.InstaData.UserName] = int64(sc.InstaData.Followers)
					}
				}
			}

			misc.WriteJSON(c, 200, out)
		} else if c.Query("yield") != "" {
			type Dummy struct {
				Yield          float64 `json:"yield"`
				ID             string  `json:"id,omitempty"`
				IsInfluencer   bool    `json:"isInfluencer,omitempty"`
				IsScrap        bool    `json:"isScrap,omitempty"`
				IsNewUser      bool    `json:"isNewUser,omitempty"`
				AvgEngagements float64 `json:"avgEngagements,omitempty"`
			}

			out := make(map[string]Dummy)

			dummyCmp := &common.Campaign{
				Budget:    1000,
				Twitter:   true,
				Facebook:  true,
				YouTube:   true,
				Instagram: true,
			}

			for _, inf := range s.auth.Influencers.GetAll() {
				maxYield := influencer.GetMaxYield(dummyCmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)
				switch platform {
				case "insta":
					if inf.Instagram != nil {
						if len(ids) > 0 && !misc.Contains(ids, inf.Instagram.UserName) {
							continue
						}

						out[strings.ToLower(inf.Instagram.UserName)] = Dummy{
							Yield:          maxYield,
							ID:             inf.Id,
							IsInfluencer:   true,
							AvgEngagements: inf.Instagram.AvgLikes + inf.Instagram.AvgComments,
						}
					}
				}
			}

			scraps := s.Scraps.GetStore()
			for _, sc := range scraps {
				maxYield := influencer.GetMaxYield(dummyCmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)
				switch platform {
				case "insta":
					if sc.InstaData != nil {
						if len(ids) > 0 && !misc.Contains(ids, sc.InstaData.UserName) {
							continue
						}

						out[strings.ToLower(sc.InstaData.UserName)] = Dummy{
							Yield:          maxYield,
							ID:             sc.Id,
							IsScrap:        true,
							AvgEngagements: sc.InstaData.AvgLikes + sc.InstaData.AvgComments,
						}
					}
				}
			}

			if len(ids) > 0 {
				// If we had IDs.. lets create users for the people that weren't in our
				// system
				for _, username := range ids {
					if username == "" {
						continue
					}
					username = strings.ToLower(username)
					if _, ok := out[username]; !ok {
						// We need to make an inf
						switch platform {
						case "insta":
							inf, err := influencer.New("", "", "", username, "", "", false, false, "", "", "", "", "", []string{}, nil, 0, s.Cfg)
							if err != nil || inf == nil || inf.Instagram == nil {
								misc.WriteJSON(c, 500, misc.StatusErr("Error for username: "+username))
								return
							}
							maxYield := influencer.GetMaxYield(dummyCmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)

							out[username] = Dummy{
								Yield:          maxYield,
								IsNewUser:      true,
								AvgEngagements: inf.Instagram.AvgLikes + inf.Instagram.AvgComments,
							}
						}
					}
				}

			}

			misc.WriteJSON(c, 200, out)
		} else {
			out := make(map[string]bool)

			for _, inf := range s.auth.Influencers.GetAll() {
				switch platform {
				case "insta":
					if inf.Instagram != nil {
						out[inf.Instagram.UserName] = true
					}
				}
			}

			scraps := s.Scraps.GetStore()
			for _, sc := range scraps {
				switch platform {
				case "insta":
					if sc.InstaData != nil {
						out[sc.InstaData.UserName] = false
					}
				}
			}

			misc.WriteJSON(c, 200, out)

		}
	}
}
