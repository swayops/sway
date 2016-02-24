package server

import (
	"encoding/json"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

func clearDeal(s *Server, dealId, influencerId, campaignId string, timeout bool) error {
	// Unssign the deal & Save the Campaign
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
			deal.InfluencerId = ""
			deal.Assigned = 0
			deal.Completed = 0
			deal.Platforms = make(map[string]float32)
			deal.AssignedPlatform = ""
			deal.AssignedPrice = 0
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

func saveInfluencer(tx *bolt.Tx, inf influencer.Influencer, cfg *config.Config) error {
	var (
		b   []byte
		err error
	)

	if b, err = json.Marshal(&inf); err != nil {
		return err
	}

	return misc.PutBucketBytes(tx, cfg.Bucket.Influencer, inf.Id, b)
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
