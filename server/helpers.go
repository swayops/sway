package server

import (
	"encoding/json"
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

//TODO discuss with Shahzil and handle scopes
func createRoutes(r *gin.Engine, srv *Server, endpoint string, scopes auth.ScopeMap, ownershipItemType auth.ItemType,
	get, post, del func(*Server) gin.HandlerFunc) {

	sh := srv.auth.CheckScopes(scopes)
	if ownershipItemType != "" {
		oh := srv.auth.CheckOwnership(ownershipItemType, "id")
		r.GET(endpoint+"/:id", srv.auth.VerifyUser, sh, oh, get(srv))
		r.POST(endpoint, srv.auth.VerifyUser, sh, post(srv))
		r.DELETE(endpoint+"/:id", srv.auth.VerifyUser, sh, oh, del(srv))
	} else {
		r.GET(endpoint+"/:id", srv.auth.VerifyUser, sh, get(srv))
		r.POST(endpoint, srv.auth.VerifyUser, sh, post(srv))
		r.DELETE(endpoint+"/:id", srv.auth.VerifyUser, sh, del(srv))
	}
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
