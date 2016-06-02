package server

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

func clearDeal(s *Server, user *auth.User, dealId, influencerId, campaignId string, timeout bool) error {
	// Unssign the deal & update the campaign and influencer buckets
	if err := s.db.Update(func(tx *bolt.Tx) (err error) {
		if user == nil || influencerId != user.ID {
			user = s.auth.GetUserTx(tx, influencerId)
		}

		var (
			inf = auth.GetInfluencer(user)
			cmp common.Campaign
		)
		if inf == nil {
			return auth.ErrInvalidUserID
		}
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
		if err = user.StoreWithData(s.auth, tx, inf); err != nil {
			return
		}

		// Save the campaign
		return saveCampaign(tx, &cmp, s)
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

func getAdvertiserFees(s *Server, advId string) (float64, float64) {
	var g *auth.Advertiser

	if err := s.db.View(func(tx *bolt.Tx) error {
		g = s.auth.GetAdvertiserTx(tx, advId)
		return nil
	}); err != nil {
		return 0, 0
	}

	return g.DspFee, g.ExchangeFee
}

func getAdvertiserFeesFromTx(a *auth.Auth, tx *bolt.Tx, advId string) (float64, float64) {
	if adv := a.GetAdvertiserTx(tx, advId); adv != nil {
		return adv.DspFee, adv.ExchangeFee
	}

	return 0, 0
}

func saveInfluencer(s *Server, tx *bolt.Tx, inf *auth.Influencer) error {
	if inf == nil || inf.Id == "" {
		return auth.ErrInvalidID
	}
	u := s.auth.GetUserTx(tx, inf.Id)
	if u == nil {
		return auth.ErrInvalidID
	}
	return u.StoreWithData(s.auth, tx, inf)
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

func (s *Server) getTalentAgencyFee(tx *bolt.Tx, id string) float64 {
	if ag := s.auth.GetTalentAgencyTx(tx, id); ag != nil {
		return ag.Fee
	}
	return 0
}

func saveAllDeals(s *Server, inf *auth.Influencer) error {
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

func getAllInfluencers(s *Server) []*auth.Influencer {
	var influencers []*auth.Influencer
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
			if inf := auth.GetInfluencer(u); inf != nil {
				influencers = append(influencers, inf)
			}
			return nil
		})
	})
	return influencers
}
