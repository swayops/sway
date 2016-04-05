package server

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
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
		if b, err = json.Marshal(cmp); err != nil {
			return err
		}
		return misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b)
	}); err != nil {
		return err
	}

	return nil
}

func getAllActiveCampaigns(s *Server) ([]*common.Campaign, map[string]struct{}) {
	// Returns a list of active campaign IDs in the system
	campaigns := map[string]struct{}{}
	campaignList := make([]*common.Campaign, 0, 512)

	if err := s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &common.Campaign{}
			if err := json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return nil
			}
			if cmp.Active && cmp.Budget > 0 && len(cmp.Deals) != 0 {
				campaigns[cmp.Id] = struct{}{}
				campaignList = append(campaignList, cmp)
			}

			return
		})
		return nil
	}); err != nil {
		log.Println("Err getting all active campaigns", err)
	}
	return campaignList, campaigns
}

func getAllInfluencers(s *Server) []*influencer.Influencer {
	influencers := make([]*influencer.Influencer, 0, 512)
	if err := s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
			inf := influencer.Influencer{}
			if err := json.Unmarshal(v, &inf); err != nil {
				log.Println("errorrrr when unmarshalling influencer", string(v))
				return nil
			}
			influencers = append(influencers, &inf)
			return
		})
		return nil
	}); err != nil {
		log.Println("Err when getting all influencers", err)
	}
	return influencers
}

func getAllActiveDeals(srv *Server) ([]*common.Deal, error) {
	// Retrieves all active deals in the system!
	var err error
	deals := []*common.Deal{}

	if err := srv.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(srv.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &common.Campaign{}
			if err = json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return err
			}

			for _, deal := range cmp.Deals {
				if deal.Assigned > 0 && deal.Completed == 0 && deal.InfluencerId != "" {
					deals = append(deals, deal)
				}
			}
			return
		})
		return nil
	}); err != nil {
		return deals, err
	}
	return deals, err
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

func saveCampaign(tx *bolt.Tx, cmp *common.Campaign, cfg *config.Config) error {
	var (
		b   []byte
		err error
	)

	if b, err = json.Marshal(cmp); err != nil {
		return err
	}

	return misc.PutBucketBytes(tx, cfg.Bucket.Campaign, cmp.Id, b)
}

func getTalentAgencyFee(s *Server, id string) float32 {
	var (
		v   []byte
		err error
		ag  common.TalentAgency
	)

	if err := s.db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(s.Cfg.Bucket.TalentAgency)).Get([]byte(id))
		return nil
	}); err != nil {
		return 0
	}

	if err = json.Unmarshal(v, &ag); err != nil {
		return 0
	}
	return ag.Fee
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
			if err := saveCampaign(tx, cmp, s.Cfg); err != nil {
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

func createRoutes(r *gin.Engine, srv *Server, endpoint string, get, post, del func(*Server) gin.HandlerFunc) {
	r.GET(endpoint+"/:id", get(srv))
	r.POST(endpoint, post(srv))
	r.DELETE(endpoint+"/:id", del(srv))
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
