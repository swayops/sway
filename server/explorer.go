package server

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

func newDealExplorer(srv *Server) error {
	ticker := time.NewTicker(srv.Cfg.ExplorerUpdate * time.Hour)
	go func() {
		if err := explore(srv); err != nil {
			log.Println("Error exploring!", err)
		}
		for range ticker.C {
			if err := explore(srv); err != nil {
				log.Println("Error exploring!", err)
			}
		}
	}()
	return nil
}

func explore(srv *Server) error {
	// Traverses active deals in our system and checks
	// to see whether they have been satisfied or have timed out
	activeDeals, err := srv.GetAllActiveDeals()
	if err != nil {
		return err
	}

	var (
		v   []byte
		inf influencer.Influencer
	)

	// The influencer has X seconds to do the deal before it's put
	// back into the pool
	minTs := int32(time.Now().Unix()) - (60 * 60 * 24 * srv.Cfg.DealTimeout)

	for _, deal := range activeDeals {
		// Go over all assigned deals in the platform
		srv.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(srv.Cfg.Bucket.Influencer)).Get([]byte(deal.InfluencerId))
			return nil
		})

		if err = json.Unmarshal(v, &inf); err != nil {
			log.Println("Failed to unmarshal influencer!")
			continue
		}

		switch deal.AssignedPlatform {
		case platform.Twitter:
			if tweet := findTwitterMatch(&inf, deal); tweet != nil {
				if err = srv.ApproveTweet(tweet, deal); err != nil {
					log.Println("Failed to approve tweet", err)
				}
			}
		case platform.Facebook:
			if post := findFacebookMatch(&inf, deal); post != nil {
				if err = srv.ApproveFacebook(post, deal); err != nil {
					log.Println("Failed to approve fb post", err)
				}
			}
		case platform.Instagram:
			if post := findInstagramMatch(&inf, deal); post != nil {
				if err = srv.ApproveInstagram(post, deal); err != nil {
					log.Println("Failed to approve instagram post", err)
				}
			}
		case platform.YouTube:
			if post := findYouTubeMatch(&inf, deal); post != nil {
				if err = srv.ApproveYouTube(post, deal); err != nil {
					log.Println("Failed to approve instagram post", err)
				}
			}
		default:
			return nil
		}

		// If the deal has not been approved and it has gone past the
		// dealTimeout.. put it back in the pool!
		if minTs > deal.Assigned {
			if err := clearDeal(srv, deal.Id, deal.InfluencerId, deal.CampaignId, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (srv *Server) GetAllActiveDeals() ([]*common.Deal, error) {
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

func (srv *Server) CompleteDeal(d *common.Deal) error {
	// Marks the deal as completed, and updates the campaign and influencer buckets
	if err := srv.db.Update(func(tx *bolt.Tx) (err error) {
		var (
			cmp *common.Campaign
			b   []byte
		)

		err = json.Unmarshal(tx.Bucket([]byte(srv.Cfg.Bucket.Campaign)).Get([]byte(d.CampaignId)), &cmp)
		if err != nil {
			return err
		}

		if !cmp.Active {
			return errors.New("Campaign is no longer active")
		}

		d.Completed = int32(time.Now().Unix())
		cmp.Deals[d.Id] = d

		var inf *influencer.Influencer
		err = json.Unmarshal(tx.Bucket([]byte(srv.Cfg.Bucket.Influencer)).Get([]byte(d.InfluencerId)), &inf)
		if err != nil {
			return err
		}

		// Add to completed deals
		if inf.CompletedDeals == nil || len(inf.CompletedDeals) == 0 {
			inf.CompletedDeals = []*common.Deal{}
		}
		inf.CompletedDeals = append(inf.CompletedDeals, d)

		// Remove from active deals
		activeDeals := []*common.Deal{}
		for _, deal := range inf.ActiveDeals {
			if deal.Id != d.Id {
				activeDeals = append(activeDeals, deal)
			}
		}
		inf.ActiveDeals = activeDeals

		// Save the Influencer
		if b, err = json.Marshal(inf); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, srv.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
			return err
		}

		// Save the campaign
		if b, err = json.Marshal(cmp); err != nil {
			return err
		}

		return misc.PutBucketBytes(tx, srv.Cfg.Bucket.Campaign, cmp.Id, b)
	}); err != nil {
		return err
	}

	return nil
}

func (srv *Server) ApproveTweet(tweet *twitter.Tweet, d *common.Deal) error {
	d.Tweet = tweet
	return srv.CompleteDeal(d)
}

func (srv *Server) ApproveFacebook(post *facebook.Post, d *common.Deal) error {
	d.Facebook = post
	return srv.CompleteDeal(d)
}

func (srv *Server) ApproveInstagram(post *instagram.Post, d *common.Deal) error {
	d.Instagram = post
	return srv.CompleteDeal(d)
}

func (srv *Server) ApproveYouTube(post *youtube.Post, d *common.Deal) error {
	d.YouTube = post
	return srv.CompleteDeal(d)
}

func findTwitterMatch(inf *influencer.Influencer, deal *common.Deal) *twitter.Tweet {
	if inf.Twitter == nil {
		return nil
	}

	for _, tw := range inf.Twitter.LatestTweets {
		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			postTags := tw.Hashtags()
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if hashtag == tg {
						foundHash = true
					}
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			for _, mt := range tw.Mentions() {
				if mt == deal.Mention {
					foundMention = true
				}
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			for _, l := range tw.Urls() {
				if l == deal.Link {
					foundLink = true
				}
			}
			if !foundLink {
				continue
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			return tw
		}
	}

	return nil
}

func findFacebookMatch(inf *influencer.Influencer, deal *common.Deal) *facebook.Post {
	if inf.Facebook == nil {
		return nil
	}

	for _, post := range inf.Facebook.LatestPosts {
		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			postTags := post.Hashtags()
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if hashtag == tg {
						foundHash = true
					}
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if strings.Contains(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if strings.Contains(post.Caption, deal.Link) {
				foundLink = true
			}

			if !foundLink {
				continue
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			return post
		}
	}

	return nil
}

func findInstagramMatch(inf *influencer.Influencer, deal *common.Deal) *instagram.Post {
	if inf.Instagram == nil {
		return nil
	}

	for _, post := range inf.Instagram.LatestPosts {
		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			for _, tg := range deal.Tags {
				for _, hashtag := range post.Hashtags {
					if strings.ToLower(hashtag) == tg {
						foundHash = true
					}
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if strings.Contains(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if strings.Contains(deal.Link, inf.Instagram.LinkInBio) {
				foundLink = true
			}

			if !foundLink {
				continue
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			return post
		}
	}

	return nil
}

func findYouTubeMatch(inf *influencer.Influencer, deal *common.Deal) *youtube.Post {
	if inf.YouTube == nil {
		return nil
	}

	for _, post := range inf.YouTube.LatestPosts {
		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			postTags := post.Hashtags()
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if strings.Contains(strings.ToLower(hashtag), tg) {
						foundHash = true
					}
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if strings.Contains(post.Description, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if strings.Contains(post.Description, deal.Link) {
				foundLink = true
			}

			if !foundLink {
				continue
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			return post
		}
	}

	return nil
}
