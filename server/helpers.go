package server

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

var ErrDealActive = errors.New("Deal is not active")

func clearDeal(s *Server, dealId, influencerId, campaignId string, timeout bool) error {
	// Unssign the active (and not complete) deal & update the campaign and influencer buckets
	inf, ok := s.auth.Influencers.Get(influencerId)
	if !ok {
		return auth.ErrInvalidUserID
	}

	if err := s.db.Update(func(tx *bolt.Tx) (err error) {

		var (
			cmp  common.Campaign
			deal *common.Deal
			ok   bool
		)
		err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(campaignId)), &cmp)
		if err != nil {
			return err
		}

		if deal, ok = cmp.Deals[dealId]; ok {
			if deal != nil && !deal.IsActive() {
				return ErrDealActive
			}

			// If there's a perk given to this deal.. lets
			// add it back to the campaign count
			if cmp.Perks != nil && deal.Perk != nil {
				// Add the count back
				cmp.Perks.Count += deal.Perk.Count

				if cmp.Perks.IsCoupon() && deal.Perk.Code != "" {
					// Add the coupon code back!
					cmp.Perks.Codes = append(cmp.Perks.Codes, deal.Perk.Code)
				}

			}

			// Flush all attribuets for the deal
			deal = deal.ConvertToClear()
			cmp.Deals[dealId] = deal
		}

		// Append to the influencer's cancellations and remove from active

		var activeDeals []*common.Deal
		for _, deal := range inf.ActiveDeals {
			if deal.Id != dealId {
				activeDeals = append(activeDeals, deal)
			}
		}

		inf.ActiveDeals = activeDeals
		if timeout {
			inf.Timeouts = append(inf.Timeouts, cmp.Id)
		} else {
			inf.Cancellations = append(inf.Cancellations, cmp.Id)
		}

		// Save the Influencer
		if err = saveInfluencer(s, tx, inf); err != nil {
			return
		}

		// Save the campaign
		return saveCampaign(tx, &cmp, s)
	}); err != nil {
		return err
	}

	return nil
}

var ErrClick = errors.New("Err shortening url")

func addDealsToCampaign(cmp *common.Campaign, s *Server, tx *bolt.Tx, spendable float64) *common.Campaign {
	var maxDeals int
	// If there are perks assigned, # of deals are capped
	// at the # of available perks

	if cmp.Perks != nil {
		if len(cmp.Deals) > 0 {
			// If a perk campaign already has deals, that means
			// we've already created max amount of deals for them,
			// so now regardless of whether or not their budget is increased,
			// or billing is ran, we need to maintain the same number of deals
			// SO LETS RETURN EARLY

			// NOTE: The only way they can have deals added is if they were
			// to add more perks!
			return cmp
		}
		maxDeals = cmp.Perks.Count
	} else if len(cmp.Whitelist) > 0 {
		if len(cmp.Deals) > 0 {
			// If a whitelist campaign already has deals, that means that
			// we've already given them the appropriate number of deals. Only way
			// they can get more deals if it they add more users to their whitelist
			return cmp
		}
		maxDeals = len(cmp.Whitelist)
	} else {
		// This function is only called on campaign creation, update and on billing day,
		// So if it's a goal based campaign, lets replenish deals!

		// Default goal of spendable divided by 5 (so 5 deals per campaign)
		maxDeals = int(spendable / 5)
	}

	if maxDeals == 0 && s.Cfg.Sandbox {
		// Override so tests don't fail based on avg engagements
		// of test users
		maxDeals = 100
	}

	return addDeals(cmp, maxDeals, s, tx)
}

func addDeals(cmp *common.Campaign, maxDeals int, s *Server, tx *bolt.Tx) *common.Campaign {
	if cmp.Deals == nil || len(cmp.Deals) == 0 {
		cmp.Deals = make(map[string]*common.Deal)
	}

	for i := 0; i < maxDeals; i++ {
		d := &common.Deal{
			Id:           misc.PseudoUUID(),
			CampaignId:   cmp.Id,
			AdvertiserId: cmp.AdvertiserId,
		}

		if cmp.Link != "" {
			// Only shorten if the
			shortenedID := common.ShortenID(d, tx, s.Cfg)
			if shortenedID == "" {
				s.Alert("Error shortening ID", ErrClick)
				continue
			}

			d.ShortenedLink = getClickUrl(shortenedID, s.Cfg)
		}

		cmp.Deals[d.Id] = d
	}

	return cmp
}

func resetDeals(cmp *common.Campaign, maxDeals int, s *Server, tx *bolt.Tx) *common.Campaign {
	cmp.Deals = make(map[string]*common.Deal)
	for i := 0; i < maxDeals; i++ {
		d := &common.Deal{
			Id:           misc.PseudoUUID(),
			CampaignId:   cmp.Id,
			AdvertiserId: cmp.AdvertiserId,
		}

		shortenedID := common.ShortenID(d, tx, s.Cfg)
		if shortenedID == "" {
			s.Alert("Error shortening ID", ErrClick)
			continue
		}

		d.ShortenedLink = getClickUrl(shortenedID, s.Cfg)
		cmp.Deals[d.Id] = d
	}

	return cmp
}

func getAdvertiserFees(a *auth.Auth, advId string) (float64, float64) {
	if g := a.GetAdvertiser(advId); g != nil {
		return g.DspFee, g.ExchangeFee
	}
	return 0, 0
}

func getAdvertiserFeesFromTx(a *auth.Auth, tx *bolt.Tx, advId string) (float64, float64) {
	if adv := a.GetAdvertiserTx(tx, advId); adv != nil {
		return adv.DspFee, adv.ExchangeFee
	}

	return 0, 0
}

func getUserImage(s *Server, data, suffix string, minW, minH int, user *auth.User) (string, error) {
	if !strings.HasPrefix(data, "data:image/") {
		return data, nil
	}

	filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.User, user.ID),
		data, user.ID, suffix, minW, minH)
	if err != nil {
		return "", err
	}

	return getImageUrl(s, s.Cfg.Bucket.User, "dash", filename, false), nil
}

func savePassword(s *Server, tx *bolt.Tx, oldPass, pass, pass2 string, user *auth.User, force bool) (bool, error) {
	if oldPass != "" && pass != "" && oldPass != pass {
		if len(pass) < 8 {
			return false, auth.ErrInvalidPass
		}
		if pass != pass2 {
			return false, auth.ErrPasswordMismatch
		}

		if err := s.auth.ChangePasswordTx(tx, user.Email, oldPass, pass, force); err != nil {
			return false, err
		}

		return true, nil
	} else {
		return false, nil
	}
}

func saveInfluencer(s *Server, tx *bolt.Tx, inf influencer.Influencer) error {
	if inf.Id == "" {
		return auth.ErrInvalidID
	}
	u := s.auth.GetUserTx(tx, inf.Id)
	if u == nil {
		return auth.ErrInvalidID
	}

	// Save in the cache
	s.auth.Influencers.SetInfluencer(inf.Id, inf)

	// Save in the DB
	return u.StoreWithData(s.auth, tx, &auth.Influencer{Influencer: &inf})
}

func updateLastEmail(s *Server, id string) error {
	inf, ok := s.auth.Influencers.Get(id)
	if !ok {
		return auth.ErrInvalidID
	}

	// Save the last email timestamp
	if err := s.db.Update(func(tx *bolt.Tx) error {
		inf.LastEmail = int32(time.Now().Unix())
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func updatePrivateEmailNotification(s *Server, id string) error {
	inf, ok := s.auth.Influencers.Get(id)
	if !ok {
		return auth.ErrInvalidID
	}

	// Save the last email timestamp
	if err := s.db.Update(func(tx *bolt.Tx) error {
		inf.PrivateNotify = int32(time.Now().Unix())
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func saveInfluencerWithUser(s *Server, tx *bolt.Tx, inf influencer.Influencer, user *auth.User) error {
	if inf.Id == "" {
		return auth.ErrInvalidID
	}

	// Save in the cache
	s.auth.Influencers.SetInfluencer(inf.Id, inf)

	// Save in the DB
	return user.Update(user).StoreWithData(s.auth, tx, &auth.Influencer{Influencer: &inf})
}

//TODO discuss with Shahzil and handle scopes
func createRoutes(r *gin.RouterGroup, srv *Server, endpoint, idName string, scopes auth.ScopeMap, ownershipItemType auth.ItemType,
	get, post, put, del func(*Server) gin.HandlerFunc) {

	sh := srv.auth.CheckScopes(scopes)
	epPlusId := endpoint + "/:" + idName
	if ownershipItemType != "" {
		oh := srv.auth.CheckOwnership(ownershipItemType, idName)
		if get != nil {
			r.GET(epPlusId, sh, oh, get(srv))
		}
		if put != nil {
			r.PUT(epPlusId, sh, oh, put(srv))
		}
		if del != nil {
			r.DELETE(epPlusId, sh, oh, del(srv))
		}
	} else {
		if get != nil {
			r.GET(epPlusId, sh, get(srv))
		}
		if put != nil {
			r.PUT(epPlusId, sh, put(srv))
		}
		if del != nil {
			r.DELETE(epPlusId, sh, del(srv))
		}
	}
	if post != nil {
		r.POST(endpoint, sh, post(srv))
	}
}

func saveCampaign(tx *bolt.Tx, cmp *common.Campaign, s *Server) error {
	var (
		b   []byte
		err error
	)

	if b, err = json.Marshal(cmp); err != nil {
		return err
	}

	if !s.Cfg.Sandbox {
		// Update the campaign store as well so things don't mess up
		// until the next cache update!
		s.Campaigns.SetActiveCampaign(cmp.Id, *cmp)
	} else {
		s.Campaigns.SetCampaign(cmp.Id, *cmp)
	}

	return misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b)
}

func (s *Server) getTalentAgencyFee(id string) float64 {
	var agencyFee float64
	if err := s.db.View(func(tx *bolt.Tx) error {
		agencyFee = s.getTalentAgencyFeeTx(tx, id)
		return nil
	}); err != nil {
		return 0
	}
	return agencyFee
}

func (s *Server) getTalentAgencyFeeTx(tx *bolt.Tx, id string) float64 {
	if ag := s.auth.GetTalentAgencyTx(tx, id); ag != nil {
		return ag.Fee
	}
	return 0
}

func saveAllCompletedDeals(s *Server, inf influencer.Influencer) error {
	// Saves the deals FROM the influencer TO the campaign!
	if err := s.db.Update(func(tx *bolt.Tx) error {
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}

		cmpB := tx.Bucket([]byte(s.Cfg.Bucket.Campaign))
		// Since we just updated the deal metrics for the influencer,
		// lets also update the deal values in the campaign
		for _, deal := range inf.CompletedDeals {
			var cmp *common.Campaign
			err := json.Unmarshal((cmpB).Get([]byte(deal.CampaignId)), &cmp)
			if err != nil {
				log.Println("Err unmarshalling campaign", err)
				continue
			}

			if _, ok := cmp.Deals[deal.Id]; ok {
				// Replace the old deal saved with the new one
				cmp.Deals[deal.Id] = deal
			}

			// Save the campaign!
			if err := saveCampaign(tx, cmp, s); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Println("Error when saving influencer", err)
		return err
	}
	return nil
}

func saveAllDeals(s *Server, inf influencer.Influencer) error {
	// Saves the deals FROM the influencer TO the campaign!
	if err := s.db.Update(func(tx *bolt.Tx) error {
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}

		cmpB := tx.Bucket([]byte(s.Cfg.Bucket.Campaign))
		// Lets update all active deals first!
		for _, deal := range inf.ActiveDeals {
			var cmp *common.Campaign
			err := json.Unmarshal((cmpB).Get([]byte(deal.CampaignId)), &cmp)
			if err != nil {
				log.Println("Err unmarshalling campaign", err)
				continue
			}
			if _, ok := cmp.Deals[deal.Id]; ok {
				// Replace the old deal saved with the new one
				cmp.Deals[deal.Id] = deal
			}

			// Save the campaign!
			if err := saveCampaign(tx, cmp, s); err != nil {
				return err
			}
		}

		// Since we just updated the deal metrics for the influencer,
		// lets also update the deal values in the campaign
		for _, deal := range inf.CompletedDeals {
			var cmp *common.Campaign
			err := json.Unmarshal((cmpB).Get([]byte(deal.CampaignId)), &cmp)
			if err != nil {
				log.Println("Err unmarshalling campaign", err)
				continue
			}
			if _, ok := cmp.Deals[deal.Id]; ok {
				// Replace the old deal saved with the new one
				cmp.Deals[deal.Id] = deal
			}

			// Save the campaign!
			if err := saveCampaign(tx, cmp, s); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Println("Error when saving influencer", err)
		return err
	}
	return nil
}

func saveAllActiveDeals(s *Server, inf influencer.Influencer) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}

		cmpB := tx.Bucket([]byte(s.Cfg.Bucket.Campaign))
		// Since we just updated the deal metrics for the influencer,
		// lets also update the deal values in the campaign
		for _, deal := range inf.ActiveDeals {
			var cmp *common.Campaign
			err := json.Unmarshal((cmpB).Get([]byte(deal.CampaignId)), &cmp)
			if err != nil {
				log.Println("Err unmarshalling campaign", err)
				continue
			}

			var foundPerksMailed bool
			if _, ok := cmp.Deals[deal.Id]; ok {
				// Replace the old deal saved with the new one
				cmp.Deals[deal.Id] = deal
				if deal.Perk != nil && deal.Perk.Status {
					foundPerksMailed = true
				}
			}

			if foundPerksMailed {
				// Lets add to timeline
				cmp.AddToTimeline(common.PERKS_MAILED, true, s.Cfg)
			}

			// Save the campaign!
			if err := saveCampaign(tx, cmp, s); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Println("Error when saving influencer", err)
		return err
	}
	return nil
}

func sanitizeMention(str string) string {
	// Removes @
	raw := strings.Map(func(r rune) rune {
		if strings.IndexRune("@", r) < 0 {
			return r
		}
		return -1
	}, str)

	return strings.ToLower(raw)
}

func sanitizeURL(incoming string) string {
	if incoming == "" {
		return ""
	}

	u, err := url.Parse(incoming)
	if err != nil {
		return ""
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}

	clean := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		clean = clean + "?" + u.RawQuery
	}
	return clean
}

func trimURLPrefix(raw string) string {
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "www.")
	return raw
}

func getAdAgencies(s *Server, tx *bolt.Tx) []*auth.AdAgency {
	var all []*auth.AdAgency
	s.auth.GetUsersByTypeTx(tx, auth.AdAgencyScope, func(u *auth.User) error {
		if ag := auth.GetAdAgency(u); ag != nil {
			all = append(all, ag)
		}
		return nil
	})
	return all
}

func getTalentAgencies(s *Server, tx *bolt.Tx) []*auth.TalentAgency {
	var all []*auth.TalentAgency

	s.auth.GetUsersByTypeTx(tx, auth.TalentAgencyScope, func(u *auth.User) error {
		if ag := auth.GetTalentAgency(u); ag != nil {
			all = append(all, ag)
		}
		return nil
	})

	return all
}

func getAdvertisers(s *Server, tx *bolt.Tx) []*auth.Advertiser {
	var advertisers []*auth.Advertiser

	s.auth.GetUsersByTypeTx(tx, auth.AdvertiserScope, func(u *auth.User) error {
		if adv := auth.GetAdvertiser(u); adv != nil {
			advertisers = append(advertisers, adv)
		}
		return nil
	})

	return advertisers
}

func getAllInfluencers(s *Server) []influencer.Influencer {
	var influencers []influencer.Influencer
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
			if inf := auth.GetInfluencer(u); inf != nil {
				influencers = append(influencers, *inf.Influencer)
			}
			return nil
		})
	})
	return influencers
}

func getActiveAdvertisers(s *Server) map[string]bool {
	out := make(map[string]bool)
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.AdvertiserScope, func(u *auth.User) error {
			adv := auth.GetAdvertiser(u)
			if adv == nil || !adv.Status {
				return nil
			}

			allowed, err := subscriptions.IsSubscriptionActive(adv.IsSelfServe(), adv.Subscription)
			if err != nil {
				s.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
				return nil
			}

			if allowed {
				out[adv.ID] = true
			}

			return nil
		})
	})
	return out
}

func getActiveAdAgencies(s *Server) map[string]bool {
	out := make(map[string]bool)
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.AdAgencyScope, func(u *auth.User) error {
			if ag := auth.GetAdAgency(u); ag != nil && ag.Status {
				out[ag.ID] = true
			}
			return nil
		})
	})
	return out
}

func getClickUrl(id string, cfg *config.Config) string {
	return cfg.ClickUrl + id
}

func Float64Frombytes(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

func Float64ToBytes(float float64) []byte {
	bits := math.Float64bits(float)
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, bits)
	return bytes
}

func isSecureAdmin(c *gin.Context, s *Server) bool {
	if c.Query("pw") == "muchodinero" || s.Cfg.Sandbox {
		return true
	} else {
		misc.WriteJSON(c, 500, misc.StatusErr("GET OUDDA HEEYAH!"))
		return false
	}
}

type DealOffer struct {
	Influencer influencer.Influencer `json:"infs,omitempty"`
	Deal       *common.Deal          `json:"deals,omitempty"`
}

func getDealsForCmp(s *Server, cmp *common.Campaign, pingOnly bool) []*DealOffer {
	campaigns := common.NewCampaigns()
	campaigns.SetCampaign(cmp.Id, *cmp)

	influencerPool := []*DealOffer{}
	for _, inf := range s.auth.Influencers.GetAll() {
		// Only email deals to people who have opted in to email deals
		// if pingOnly is true
		if pingOnly && !inf.DealPing {
			continue
		}

		deals, _ := inf.GetAvailableDeals(campaigns, s.Audiences, s.db, "", "", nil, false, s.Cfg)
		if len(deals) == 0 {
			continue
		}

		// NOTE: Add this check once we have a good pool of influencers!
		// You have a 25% chance of getting an email.. and your odds
		// are higher if you have done or are doing a deal!
		// if rand.Intn(100) > (25 + len(inf.ActiveDeals) + len(inf.CompleteDeals)) {
		// 	return nil
		// }
		influencerPool = append(influencerPool, &DealOffer{inf, deals[0]})
	}

	return influencerPool
}

type ForecastUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`

	ProfilePicture  string `json:"profilePicture"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	Followers       int64  `json:"followers"`
	StringFollowers string `json:"stringFollowers"`
	AvgEngs         int64  `json:"avgEngs"`

	// Float representation of max yield
	Rate float64 `json:"rate"`
	// Used for display purposes in reports
	MaxYield   string `json:"maxYield"`
	Geo        string `json:"geo"`
	Gender     string `json:"gender"`
	Categories string `json:"categories"`

	HasTwitter      bool   `json:"hasTwitter"`
	TwitterUsername string `json:"twUsername"`

	HasInsta      bool   `json:"hasInsta"`
	InstaUsername string `json:"instaUsername"`

	HasYoutube      bool   `json:"hasYoutube"`
	YoutubeUsername string `json:"ytUsername"`

	HasFacebook      bool   `json:"hasFacebook"`
	FacebookUsername string `json:"fbUsername"`
}

func (user *ForecastUser) IsProfilePictureActive() bool {
	if user.ProfilePicture == "" || (user.ProfilePicture != "" && misc.Ping(user.ProfilePicture) != nil) {
		return false
	}
	return true
}

func getForecastForCmp(s *Server, cmp common.Campaign, sortBy string) (influencers []*ForecastUser, reach int64) {
	// Some easy bail outs
	if !cmp.Instagram && !cmp.Twitter && !cmp.YouTube && !cmp.Facebook {
		return
	}

	if !cmp.Male && !cmp.Female {
		return
	}

	// Lite version of the original GetAVailableDeals just for forecasting
	// spendable := budget.GetProratedBudget(cmp.Budget)

	// Calculate max deals for this campaign
	// var maxDeals int
	// if cmp.Perks != nil {
	// 	maxDeals = cmp.Perks.Count
	// } else if len(cmp.Whitelist) > 0 {
	// 	maxDeals = len(cmp.Whitelist)
	// } else {
	// 	if cmp.Goal > 0 {
	// 		if cmp.Goal > spendable {
	// 			maxDeals = 1
	// 		} else {
	// 			maxDeals = int(spendable / cmp.Goal)
	// 		}
	// 	} else {
	// 		// Default goal of spendable divided by 5 (so 5 deals per campaign)
	// 		maxDeals = int(spendable / 5)
	// 	}
	// }

	// target := spendable / float64(maxDeals)
	// margin := 0.3 * target

	// Pre calculate target yield
	// min, max := target-margin, target+margin
	infs := s.auth.Influencers.GetAll()
	for _, inf := range infs {
		if inf.IsBanned() {
			continue
		}

		if len(cmp.Categories) > 0 || len(cmp.Audiences) > 0 {
			catFound := false
		L1:
			for _, cat := range cmp.Categories {
				for _, infCat := range inf.Categories {
					if infCat == cat {
						catFound = true
						break L1
					}
				}
			}

			// Audience check!
			if !catFound {
				for _, targetAudience := range cmp.Audiences {
					if s.Audiences.IsAllowed(targetAudience, inf.EmailAddress) {
						// There was an audience target and they're in it
						catFound = true
						break
					}
				}
			}

			if !catFound {
				continue
			}
		}

		if len(cmp.Keywords) > 0 {
			kwFound := false
		L2:
			for _, kw := range cmp.Keywords {
				for _, infKw := range inf.Keywords {
					if common.IsExactMatch(kw, infKw) {
						kwFound = true
						break L2
					}
				}

				if inf.Instagram != nil && inf.Instagram.Bio != "" {
					if common.IsExactMatch(inf.Instagram.Bio, kw) {
						kwFound = true
						break L2
					}
				}
			}
			if !kwFound {
				continue
			}
		}

		if !geo.IsGeoMatch(cmp.Geos, inf.GetLatestGeo()) {
			continue
		}

		// Gender check
		if !cmp.Male && cmp.Female && !inf.Female {
			// Only want females
			continue
		} else if cmp.Male && !cmp.Female && !inf.Male {
			// Only want males
			continue
		} else if !cmp.Male && !cmp.Female {
			continue
		}

		_, ok := cmp.Whitelist[inf.EmailAddress]
		if ok {
			// This person is already in the campaign!
			continue
		}

		// MAX YIELD
		maxYield := influencer.GetMaxYield(&cmp, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)
		// if !cmp.IsProductBasedBudget() && len(cmp.Whitelist) == 0 && !s.Cfg.Sandbox {
		// 	// NOTE: Skip this for whitelisted campaigns!

		// 	// OPTIMIZATION: Goal is to distribute funds evenly
		// 	// given what the campaign's influencer goal is and how
		// 	// many funds we have left
		// 	if maxYield < min || maxYield > max || maxYield == 0 {
		// 		continue
		// 	}
		// }

		user := &ForecastUser{
			ID:          inf.Id,
			Name:        strings.Title(inf.Name),
			Email:       inf.EmailAddress,
			AvgEngs:     inf.GetAvgEngs(),
			Followers:   inf.GetFollowers(),
			Description: inf.GetDescription(),
			MaxYield:    fmt.Sprintf("$%0.2f", maxYield),
			Rate:        maxYield,
			Geo:         "N/A",
			Gender:      "N/A",
			Categories:  "N/A",
		}
		user.StringFollowers = common.Commanize(user.Followers)

		if geo := inf.GetLatestGeo(); geo != nil {
			if geo.State != "" && geo.Country != "" {
				user.Geo = geo.State + ", " + geo.Country
			} else if geo.State == "" && geo.Country != "" {
				user.Geo = geo.Country
			}
		}

		if inf.Male {
			user.Gender = "M"
		} else if inf.Female {
			user.Gender = "F"
		}

		if len(inf.Categories) > 0 {
			user.Categories = strings.Join(inf.Categories, ", ")
		}

		// Social Media Checks
		socialMediaFound := false
		if cmp.YouTube && inf.YouTube != nil {
			socialMediaFound = true
			if inf.YouTube.ProfilePicture != "" {
				user.ProfilePicture = inf.YouTube.ProfilePicture
			}
			user.URL = inf.YouTube.GetProfileURL()
			user.HasYoutube = true
			user.YoutubeUsername = inf.YouTube.UserName
		}

		if cmp.Instagram && inf.Instagram != nil {
			socialMediaFound = true
			if inf.Instagram.ProfilePicture != "" {
				user.ProfilePicture = inf.Instagram.ProfilePicture
			}
			user.URL = inf.Instagram.GetProfileURL()
			user.HasInsta = true
			user.InstaUsername = inf.Instagram.UserName
		}

		if cmp.Twitter && inf.Twitter != nil {
			socialMediaFound = true
			if inf.Twitter.ProfilePicture != "" {
				user.ProfilePicture = inf.Twitter.ProfilePicture
			}
			user.URL = inf.Twitter.GetProfileURL()
			user.HasTwitter = true
			user.TwitterUsername = inf.Twitter.Id
		}

		if cmp.Facebook && inf.Facebook != nil {
			socialMediaFound = true

			if inf.Facebook.ProfilePicture != "" {
				user.ProfilePicture = inf.Facebook.ProfilePicture
			}
			user.URL = inf.Facebook.GetProfileURL()
			user.HasFacebook = true
			user.FacebookUsername = inf.Facebook.Id
		}

		if !socialMediaFound {
			continue
		}

		// Lets check to see a match on eng, follower, price targeting now that
		// we have those values
		// NOTE: For scraps this is done within the Match func
		if cmp.EngTarget != nil && !cmp.EngTarget.InRange(user.AvgEngs) {
			continue
		}

		if cmp.FollowerTarget != nil && !cmp.FollowerTarget.InRange(user.Followers) {
			continue
		}

		// Lets see if max yield falls into target range for the campaign
		if cmp.PriceTarget != nil && cmp.PriceTarget.InRange(maxYield) {
			continue
		}

		influencers = append(influencers, user)
	}

	// Lets go over scraps now!
	scraps := s.Scraps.GetStore()

	tmpStore, _ := budget.GetCampaignStoreFromDb(s.db, s.Cfg, cmp.Id, cmp.AdvertiserId)

	scrapUsers := []*ForecastUser{}
	for _, sc := range scraps {
		if sc.Match(cmp, s.Audiences, s.db, s.Cfg, tmpStore, true) {
			_, ok := cmp.Whitelist[sc.EmailAddress]
			if ok {
				// This person is already in the campaign!
				continue
			}

			user := &ForecastUser{
				ID:          "sc-" + sc.Id,
				Name:        strings.Title(sc.Name),
				Email:       sc.EmailAddress,
				AvgEngs:     sc.GetAvgEngs(),
				Followers:   sc.GetFollowers(),
				Description: sc.GetDescription(),
				Geo:         "N/A",
				Gender:      "N/A",
				Categories:  "N/A",
			}
			user.Rate = influencer.GetMaxYield(&cmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)
			user.MaxYield = fmt.Sprintf("$%0.2f", user.Rate)
			user.StringFollowers = common.Commanize(user.Followers)

			if geo := sc.Geo; geo != nil {
				if geo.State != "" && geo.Country != "" {
					user.Geo = geo.State + ", " + geo.Country
				} else if geo.State == "" && geo.Country != "" {
					user.Geo = geo.Country
				}
			}

			if sc.Male {
				user.Gender = "M"
			} else if sc.Female {
				user.Gender = "F"
			}

			if len(sc.Categories) > 0 {
				user.Categories = strings.Join(sc.Categories, ", ")
			}

			if sc.FBData != nil {
				if sc.FBData.ProfilePicture != "" {
					user.ProfilePicture = sc.FBData.ProfilePicture
				}
				user.URL = sc.FBData.GetProfileURL()
				user.HasFacebook = true
				user.FacebookUsername = sc.FBData.Id
			}

			if sc.InstaData != nil {
				if sc.InstaData.ProfilePicture != "" {
					user.ProfilePicture = sc.InstaData.ProfilePicture
				}
				user.URL = sc.InstaData.GetProfileURL()
				user.HasInsta = true
				user.InstaUsername = sc.InstaData.UserName
			}

			if sc.TWData != nil {
				if sc.TWData.ProfilePicture != "" {
					user.ProfilePicture = sc.TWData.ProfilePicture
				}
				user.URL = sc.TWData.GetProfileURL()
				user.HasTwitter = true
				user.TwitterUsername = sc.TWData.Id
			}

			if sc.YTData != nil {
				if sc.YTData.ProfilePicture != "" {
					user.ProfilePicture = sc.YTData.ProfilePicture
				}
				user.URL = sc.YTData.GetProfileURL()
				user.HasYoutube = true
				user.YoutubeUsername = sc.YTData.UserName
			}

			scrapUsers = append(scrapUsers, user)
		}
	}

	// Shuffle the scrap users IF theres no sort by
	if sortBy == "" {
		for i := range scrapUsers {
			j := rand.Intn(i + 1)
			scrapUsers[i], scrapUsers[j] = scrapUsers[j], scrapUsers[i]
		}
	}

	// Now that we POTENTIALLY shuffled scrap users.. add them to the influencers array
	influencers = append(influencers, scrapUsers...)

	// Get reach
	for _, i := range influencers {
		reach += i.Followers
	}

	// Lets calculate count and reach now
	switch sortBy {
	case "engagements":
		sort.Slice(influencers, func(i int, j int) bool {
			return influencers[i].AvgEngs > influencers[j].AvgEngs
		})
	case "followers":
		sort.Slice(influencers, func(i int, j int) bool {
			return influencers[i].Followers > influencers[j].Followers
		})
	}

	return
}

var ErrWhitelist = errors.New("This campaign has a whitelist!")

func emailDeal(s *Server, cid string) (bool, error) {
	cmp := common.GetCampaign(cid, s.db, s.Cfg)
	if cmp == nil {
		log.Println("Cannot find campaign when emailing deal", cid)
		return false, ErrCampaign
	}

	if len(cmp.Whitelist) > 0 {
		// If this camaign has a whitelist.. use
		// emailList!
		return false, ErrWhitelist
	}

	influencerPool := getDealsForCmp(s, cmp, true)

	// Shuffle the pool if it's a normal campaign!
	for i := range influencerPool {
		// Lets see which of these dudes is on the whitelist and we'll make sure
		// to email those first!
		j := rand.Intn(i + 1)
		influencerPool[i], influencerPool[j] = influencerPool[j], influencerPool[i]
	}

	emailed := 0
	for _, offer := range influencerPool {
		if emailed >= len(cmp.Deals) {
			break
		}

		err := offer.Influencer.EmailDeal(offer.Deal, s.Cfg)
		if err != nil {
			continue
		}

		if err = updateLastEmail(s, offer.Influencer.Id); err != nil {
			continue
		}

		emailed += 1
	}

	s.Notify(
		fmt.Sprintf("Emailed %d influencers for campaign %s (%s)", emailed, cmp.Name, cid),
		fmt.Sprintf("Sway has successfully emailed %d influencers for campaign %s (%s)!", emailed, cmp.Name, cid),
	)

	return true, nil
}

func emailList(s *Server, cid string, override []string) {
	cmp := common.GetCampaign(cid, s.db, s.Cfg)
	if cmp == nil {
		log.Println("Cannot find campaign", cid)
		return
	}

	var list []string
	if len(override) > 0 {
		list = override
	} else {
		list = common.SliceWhitelist(cmp.Whitelist)
	}

	if len(list) > 0 {
		// NOTE: Emailing generic emails means that there's
		// a chance people get emails for deals they aren't
		// eligible for due to cmp filters!

		// All the required attributes for the email!
		genericDeal := &common.Deal{
			CampaignName:  cmp.Name,
			CampaignImage: cmp.ImageURL,
			Company:       cmp.Company,
			CampaignId:    cmp.Id, // Added for logging purposes
		}

		for _, email := range list {
			// Email everyone in whitelist!
			inf := &influencer.Influencer{
				EmailAddress: misc.TrimEmail(email),
			}

			inf.Id = inf.EmailAddress // For logging purposes

			err := inf.EmailDeal(genericDeal, s.Cfg)
			if err != nil {
				log.Println("Error emailing for new campaign WL!", cid, err, inf.EmailAddress)
				continue
			}
		}
	} else {
		// This would be hit in the case that the campaign
		// was taken OFF a whitelist in the time.Sleep of 1 hour
		// before emailList was hit!
		emailDeal(s, cmp.Id)
	}

	s.Notify(
		fmt.Sprintf("Emailed %d whitelisted influencers for campaign %s", len(list), cid),
		fmt.Sprintf("Sway has successfully emailed %d whitelisted influencers for campaign %s!", len(list), cid),
	)
}

func emailStatusUpdate(s *Server, cid string) {
	// Emails status updates to the influencers with
	// active deals for this campaign
	cmp := common.GetCampaign(cid, s.db, s.Cfg)
	if cmp == nil {
		log.Println("Cannot find campaign", cid)
		return
	}

	if cmp.Status {
		// The campaign was turned back on!
		return
	}

	var count int32
	// Go over all deals and find any active ones!
	for _, deal := range cmp.Deals {
		if deal.IsActive() {
			// We need to tell the influencer for this deal
			// that it's been set to off now!
			inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
			if !ok {
				continue
			}

			if err := clearDeal(s, deal.Id, inf.Id, cmp.Id, false); err != nil {
				s.Alert("Failed to clear deal for influencer: "+inf.Id, err)
				continue
			}

			if err := inf.DealUpdate(cmp, s.Cfg); err != nil {
				s.Alert("Failed to give influencer a campaign update: "+inf.Id, err)
				continue
			}

			count += 1
		}
	}

	s.Notify(
		fmt.Sprintf("Cleared out deals for %d  influencers for campaign %s", count, cmp.Id),
		fmt.Sprintf("Sway has successfully cleared out %d deals for campaign %s!", count, cmp.Id),
	)
}

func assignDealEmail(s *Server, cmp *common.Campaign, deal *common.Deal, inf *influencer.Influencer) {
	// Emails influencer's with deal instructions
	if err := inf.DealInstructions(cmp, deal, s.Cfg); err != nil {
		s.Alert("Failed to give influencer deal instructions: "+inf.Id, err)
	}
}

// saveUserImage saves the user image to disk and sets User.ImageURL to the url for it if the image is a data:image/
func saveUserImage(s *Server, u *auth.User) error {
	if strings.HasPrefix(u.ImageURL, "data:image/") {

		filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.User, u.ID), u.ImageURL, u.ID, "", 300, 300)
		if err != nil {
			return err
		}

		u.ImageURL = getImageUrl(s, s.Cfg.Bucket.User, "dash", filename, false)
	}

	if strings.HasPrefix(u.CoverImageURL, "data:image/") {
		filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.User, u.ID),
			u.CoverImageURL, u.ID, "-cover", 300, 300)
		if err != nil {
			return err
		}

		u.CoverImageURL = getImageUrl(s, s.Cfg.Bucket.User, "dash", filename, false)
	}

	return nil
}

func saveUserHelper(s *Server, c *gin.Context, userType string) {
	var (
		incUser struct {
			auth.User
			// support changing passwords
			OldPass string `json:"oldPass"`
			Pass    string `json:"pass"`
			Pass2   string `json:"pass2"`
		}
		user       = auth.GetCtxUser(c)
		id         = c.Param("id")
		su         auth.SpecUser
		changePass = false
	)

	if err := misc.BindJSON(c, &incUser); err != nil {
		misc.AbortWithErr(c, 400, err)
		return
	}

	switch userType {
	case "advertiser":
		su = incUser.Advertiser
	case "adAgency":
		su = incUser.AdAgency
	case "talentAgency":
		su = incUser.TalentAgency
	case "admin":

	}

	if su == nil && userType != "admin" {
		misc.AbortWithErr(c, 400, auth.ErrInvalidRequest)
		return
	}

	if incUser.OldPass != "" && incUser.Pass != "" && incUser.OldPass != incUser.Pass {
		if len(incUser.Pass) < 8 {
			misc.AbortWithErr(c, 400, auth.ErrInvalidPass)
			return
		}
		if incUser.Pass != incUser.Pass2 {
			misc.AbortWithErr(c, 400, auth.ErrPasswordMismatch)
			return
		}
		changePass = true
	}

	if incUser.ID == "" {
		incUser.ID = id // for saveImage
	}

	if err := saveUserImage(s, &incUser.User); err != nil {
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
		if changePass {
			email := incUser.Email
			if subEmail := auth.SubUser(c); subEmail != "" {
				email = subEmail
			}
			if err := s.auth.ChangePasswordTx(tx, email, incUser.OldPass, incUser.Pass, false); err != nil {
				return err
			}
			user = s.auth.GetUserTx(tx, id) // always reload after changing the password
		}

		if adv := incUser.Advertiser; adv != nil && adv.SubLoad != nil && user.Advertiser != nil {
			if incUser.Advertiser.SubLoad.Plan != user.Advertiser.Plan {
				// The plan was updated! Lets copy over the plan to all child campaigns
				targetPlan := incUser.Advertiser.SubLoad.Plan
				tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
					cmp := &common.Campaign{}
					if err := json.Unmarshal(v, cmp); err != nil {
						log.Printf("error when unmarshalling campaign %s: %v", v, err)
						return nil
					}

					if cmp.AdvertiserId != user.Advertiser.ID {
						// If advertiser ID is different.. BAIL!
						return
					}

					// Change the plan!
					cmp.Plan = targetPlan

					// Save the campaign!
					if err := saveCampaign(tx, cmp, s); err != nil {
						return err
					}
					return
				})
			}
		}

		if su == nil { // admin
			return user.Update(&incUser.User).Store(s.auth, tx)
		}
		return user.Update(&incUser.User).StoreWithData(s.auth, tx, su)
	}); err != nil {
		misc.AbortWithErr(c, 400, err)
		return
	}

	misc.WriteJSON(c, 200, misc.StatusOK(id))
}

func getAllCampaigns(db *bolt.DB, cfg *config.Config) []*common.Campaign {
	// Returns a list of ALL campaigns in the system
	campaignList := []*common.Campaign{}

	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &common.Campaign{}
			if err := json.Unmarshal(v, cmp); err != nil {
				log.Printf("error when unmarshalling campaign %s: %v", v, err)
				return nil
			}

			campaignList = append(campaignList, cmp)

			return
		})
		return nil
	}); err != nil {
		log.Println("Err getting all active campaigns", err)
	}

	return campaignList
}

func getPerkHandout(d *common.Deal, cmp *common.Campaign) string {
	var capitalPlatforms []string
	for _, pl := range d.Platforms {
		capitalPlatforms = append(capitalPlatforms, strings.Title(pl))
	}

	return templates.Handout.Render(map[string]interface{}{"Name": d.InfluencerName, "Company": cmp.Company, "Task": d.Task, "Instructions": d.GetInstructions(), "Platforms": capitalPlatforms})
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

func getAllCategories(s *Server) []*InfCategory {
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

	// Lets go over scraps now!
	scraps := s.Scraps.GetStore()
	for _, sc := range scraps {
		for _, cat := range sc.Categories {
			if val := findCat(out, cat); val != nil {
				val.Influencers += 1
				val.Reach += sc.Followers
			}
		}
	}

	return out
}

func getFollowersByEmail(s *Server) map[string]int64 {
	byEmail := make(map[string]int64)
	for _, inf := range s.auth.Influencers.GetAll() {
		byEmail[inf.EmailAddress] = inf.GetFollowers()
	}

	// Lets go over scraps now!
	scraps := s.Scraps.GetStore()

	for _, sc := range scraps {
		byEmail[sc.EmailAddress] = sc.Followers
	}

	return byEmail
}
