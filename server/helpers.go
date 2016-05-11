package server

import (
	"encoding/json"
	"log"
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
	if err := s.db.Update(func(tx *bolt.Tx) (err error) {
		var (
			cmp *common.Campaign
			b   []byte
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
		var inf *influencer.Influencer
		err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(influencerId)), &inf)
		if err != nil {
			return err
		}

		activeDeals := []*common.Deal{}
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
		if b, err = json.Marshal(inf); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
			return err
		}

		// Save the campaign
		return saveCampaign(tx, cmp, s)
	}); err != nil {
		return err
	}

	return nil
}

func addDealsToCampaign(cmp *common.Campaign) *common.Campaign {
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

	// Budget is always monthly
	// Keeping it low because acceptance rate is low
	maxDeals := int(cmp.Budget / 1.5)
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

func getAdvertiserFees(s *Server, advId string) (float32, float32) {
	var (
		g   common.Advertiser
		v   []byte
		err error
	)

	if err = s.db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(advId))
		return nil
	}); err != nil {
		return 0, 0
	}

	if err = json.Unmarshal(v, &g); err != nil {
		return 0, 0
	}

	return g.DspFee, g.ExchangeFee
}

func getAdvertiserFeesFromTx(tx *bolt.Tx, cfg *config.Config, advId string) (float32, float32) {
	var (
		g   common.Advertiser
		v   []byte
		err error
	)

	v = tx.Bucket([]byte(cfg.Bucket.Advertiser)).Get([]byte(advId))

	if err = json.Unmarshal(v, &g); err != nil {
		return 0, 0
	}

	return g.DspFee, g.ExchangeFee
}

func getInfluencerFromId(s *Server, id string) (*influencer.Influencer, error) {

	var (
		v   []byte
		err error
		g   influencer.Influencer
	)

	if err := s.db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id))
		return nil
	}); err != nil {
		return &g, err
	}

	if err = json.Unmarshal(v, &g); err != nil {
		return &g, err
	}

	return &g, nil
}

func saveInfluencer(tx *bolt.Tx, inf *influencer.Influencer, cfg *config.Config) error {
	var (
		b   []byte
		err error
	)

	if b, err = json.Marshal(inf); err != nil {
		return err
	}

	return misc.PutBucketBytes(tx, cfg.Bucket.Influencer, inf.Id, b)
}

//TODO discuss with Shahzil and handle scopes
func createRoutes(r *gin.Engine, srv *Server, endpoint string, scopes auth.ScopeMap, ownershipItemType auth.ItemType,
	get, post, put, del func(*Server) gin.HandlerFunc) {

	sh := srv.auth.CheckScopes(scopes)
	if ownershipItemType != "" {
		oh := srv.auth.CheckOwnership(ownershipItemType, "id")
		r.GET(endpoint+"/:id", srv.auth.VerifyUser(false), sh, oh, get(srv))
		r.POST(endpoint, srv.auth.VerifyUser(false), sh, post(srv))
		r.PUT(endpoint+"/:id", srv.auth.VerifyUser(false), sh, oh, put(srv))
		r.DELETE(endpoint+"/:id", srv.auth.VerifyUser(false), sh, oh, del(srv))
	} else {
		r.GET(endpoint+"/:id", srv.auth.VerifyUser(false), sh, get(srv))
		r.POST(endpoint, srv.auth.VerifyUser(false), sh, post(srv))
		r.PUT(endpoint+"/:id", srv.auth.VerifyUser(false), sh, put(srv))
		r.DELETE(endpoint+"/:id", srv.auth.VerifyUser(false), sh, del(srv))
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

func (s *Server) getTalentAgencyFee(tx *bolt.Tx, id string) float32 {
	if ag := s.auth.GetTalentAgencyTx(tx, id); ag != nil {
		return ag.Fee
	}
	return 0
}

func saveAllDeals(s *Server, inf *influencer.Influencer) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		// Save the influencer since we just updated it's social media data
		if err := saveInfluencer(tx, inf, s.Cfg); err != nil {
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

func lowerArr(s []string) []string {
	for i, v := range s {
		s[i] = strings.ToLower(v)
	}
	return s
}
