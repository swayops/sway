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
	"regexp"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
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
			log.Println("Emailing influencer error", err)
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

var emailReg = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\s*?\b`)

func stripEmail(str string) string {
	return emailReg.ReplaceAllString(str, "")
}

func emailAdvertiser(s *Server, user *auth.User, content, subject string) {
	// Get list of users and their names
	if s.Cfg.ReplyMailClient() == nil {
		return
	}

	// Holding email addresses we need to email
	emails := []string{user.Email}
	s.db.View(func(tx *bolt.Tx) error {
		emails = append(emails, s.auth.ListSubUsersTx(tx, user.Advertiser.ID)...)
		return nil
	})

	// Also add agency email we need to email
	agency := s.auth.GetUser(user.Advertiser.AgencyID)
	if agency != nil && agency.AdAgency != nil && strings.EqualFold(agency.Email, AdAdminEmail) {
		emails = append(emails, agency.Email)
	}

	for _, email := range emails {
		resp, err := s.Cfg.ReplyMailClient().SendMessage(content, subject, email, user.Name, []string{""})
		if err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			s.Alert(fmt.Sprintf("Failed to mail advertiser subject (%s): %s", subject, email), err)
		} else {
			if err := s.Cfg.Loggers.Log("email", map[string]interface{}{
				"tag": subject,
				"id":  email,
			}); err != nil {
				log.Println("Failed to log email notification!", user.ID, subject)
			}
		}
	}

}
