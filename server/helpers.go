package server

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/url"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

func clearDeal(s *Server, dealId, influencerId, campaignId string, timeout bool) error {
	// Unssign the deal & update the campaign and influencer buckets
	inf, ok := s.auth.Influencers.Get(influencerId)
	if inf == nil || !ok {
		return auth.ErrInvalidUserID
	}

	if err := s.db.Update(func(tx *bolt.Tx) (err error) {

		var (
			cmp common.Campaign
		)
		err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(campaignId)), &cmp)
		if err != nil {
			return err
		}

		if deal, ok := cmp.Deals[dealId]; ok {
			// Flush all attribuets for the deal
			deal.InfluencerId = ""
			deal.Assigned = 0
			deal.Completed = 0
			deal.Platforms = []string{}
			deal.AssignedPlatform = ""
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
			inf.Timeouts += 1
		} else {
			inf.Cancellations += 1
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

func addDealsToCampaign(cmp *common.Campaign, spendable float64) *common.Campaign {
	// Assuming each deal will be paying out max of $5
	// Lower this if you want less deals

	// The number of deals created is based on an avg
	// pay per deal value. These deals will be the pool
	// available.. no more. The deals are later checked
	// in GetAvailableDeals function to see if they have
	// been assigned and if they are eligible for the
	// given influencer.

	if cmp.Deals == nil || len(cmp.Deals) == 0 {
		cmp.Deals = make(map[string]*common.Deal)
	}

	var maxDeals int
	// If there are perks assigned, # of deals are capped
	// at the # of available perks

	if cmp.Perks != nil {
		maxDeals = cmp.Perks.Count
	} else {
		// Budget is always monthly
		// Keeping it low because acceptance rate is low
		maxDeals = int(spendable / 1.5)
	}

	for i := 0; i < maxDeals; i++ {
		d := &common.Deal{
			Id:           misc.PseudoUUID(),
			CampaignId:   cmp.Id,
			AdvertiserId: cmp.AdvertiserId,
		}
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

func saveInfluencer(s *Server, tx *bolt.Tx, inf *influencer.Influencer) error {
	if inf == nil || inf.Id == "" {
		return auth.ErrInvalidID
	}
	u := s.auth.GetUserTx(tx, inf.Id)
	if u == nil {
		return auth.ErrInvalidID
	}

	// Save in the cache
	s.auth.Influencers.SetInfluencer(inf.Id, inf)

	// Save in the DB
	return u.StoreWithData(s.auth, tx, &auth.Influencer{inf})
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

	// Update the campaign store as well so things don't mess up
	// until the next cache update!
	s.Campaigns.SetCampaign(cmp.Id, cmp)

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

func saveAllCompletedDeals(s *Server, inf *influencer.Influencer) error {
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

func saveAllActiveDeals(s *Server, inf *influencer.Influencer) error {
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

func sanitizeHash(str string) string {
	// Removes #
	raw := strings.Map(func(r rune) rune {
		if strings.IndexRune("#", r) < 0 {
			return r
		}
		return -1
	}, str)

	return strings.ToLower(raw)
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

func getAllInfluencers(s *Server) []*influencer.Influencer {
	var influencers []*influencer.Influencer
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
			if inf := auth.GetInfluencer(u); inf != nil {
				influencers = append(influencers, inf.Influencer)
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
			if adv := auth.GetAdvertiser(u); adv != nil && adv.Status {
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

func getClickUrl(infId string, deal *common.Deal, cfg *config.Config) string {
	return cfg.ClickUrl + infId + "/" + deal.CampaignId + "/" + deal.Id
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
		c.JSON(500, misc.StatusErr("GET OUDDA HEEYAH!"))
		return false
	}
}

type dealOffer struct {
	Influencer *influencer.Influencer
	Deal       *common.Deal
}

func emailDeal(s *Server, cmp *common.Campaign) (bool, error) {
	campaigns := common.NewCampaigns()
	campaigns.SetCampaign(cmp.Id, cmp)

	influencerPool := []*dealOffer{}
	for _, inf := range s.auth.Influencers.GetAll() {
		// Only email deals to people who have opted in to email deals
		if !inf.DealPing {
			continue
		}

		deals := inf.GetAvailableDeals(campaigns, s.budgetDb, "", nil, false, s.Cfg)
		if len(deals) == 0 {
			continue
		}

		// NOTE: Add this check once we have a good pool of influencers!
		// You have a 25% chance of getting an email.. and your odds
		// are higher if you have done or are doing a deal!
		// if rand.Intn(100) > (25 + len(inf.ActiveDeals) + len(inf.CompleteDeals)) {
		// 	return nil
		// }
		influencerPool = append(influencerPool, &dealOffer{inf, deals[0]})

	}

	// Shuffle the pool if it's a normal campaign!
	for i := range influencerPool {
		// Lets see which of these dudes is on the whitelist and we'll make sure
		// to email those first!
		j := rand.Intn(i + 1)
		influencerPool[i], influencerPool[j] = influencerPool[j], influencerPool[i]
	}

	emailed := 0
	for _, offer := range influencerPool {
		if emailed >= 50 && len(cmp.Whitelist) == 0 {
			// Email no more than 50 UNLESS there's a whitelist!
			break
		}

		err := offer.Influencer.EmailDeal(offer.Deal, s.Cfg)
		if err != nil {
			log.Println("Error emailing for new campaign!")
			continue
		}

		emailed += 1
	}

	return true, nil
}

func emailList(s *Server, cmp *common.Campaign, emails []string) {
	if len(emails) > 0 {
		// NOTE: Emailing generic emails means that there's
		// a chance people get emails for deals they aren't
		// eligible for due to cmp filters!

		// All the required attributes for the email!
		genericDeal := &common.Deal{
			CampaignName:  cmp.Name,
			CampaignImage: cmp.ImageURL,
			Company:       cmp.Company,
		}

		for _, email := range emails {
			// Email everyone in whitelist!
			inf := &influencer.Influencer{
				EmailAddress: misc.TrimEmail(email),
			}
			inf.EmailDeal(genericDeal, s.Cfg)
		}
	}

}
