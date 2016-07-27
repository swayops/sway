package server

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

var requiredHash = []string{"ad", "promotion", "sponsored", "sponsoredPost", "paidPost", "endorsement"}

func explore(srv *Server) error {
	// Traverses active deals in our system and checks
	// to see whether they have been satisfied or have timed out
	activeDeals, err := common.GetAllActiveDeals(srv.db, srv.Cfg)
	if err != nil {
		return err
	}

	// The influencer has X seconds to do the deal before it's put
	// back into the pool
	minTs := int32(time.Now().Unix()) - (60 * 60 * 24 * srv.Cfg.DealTimeout)

	for _, deal := range activeDeals {
		// Go over all assigned deals in the platform
		var inf *auth.Influencer
		srv.db.View(func(tx *bolt.Tx) error {
			inf = srv.auth.GetInfluencerTx(tx, deal.InfluencerId)
			return nil
		})

		if inf == nil {
			log.Println("Failed to unmarshal influencer!")
			continue
		}

		if deal.Perk != nil && !deal.Perk.Status {
			// Perk hasn't been sent!
			continue
		}

		switch deal.AssignedPlatform {
		case platform.Twitter:
			if tweet := findTwitterMatch(inf, deal); tweet != nil {
				if err = srv.ApproveTweet(tweet, deal); err != nil {
					log.Println("Failed to approve tweet", err)
				}
			}
		case platform.Facebook:
			if post := findFacebookMatch(inf, deal); post != nil {
				if err = srv.ApproveFacebook(post, deal); err != nil {
					log.Println("Failed to approve fb post", err)
				}
			}
		case platform.Instagram:
			if post := findInstagramMatch(inf, deal); post != nil {
				if err = srv.ApproveInstagram(post, deal); err != nil {
					log.Println("Failed to approve instagram post", err)
				}
			}
		case platform.YouTube:
			if post := findYouTubeMatch(inf, deal); post != nil {
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
			if err := clearDeal(srv, nil, deal.Id, deal.InfluencerId, deal.CampaignId, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (srv *Server) CompleteDeal(d *common.Deal) error {
	// Marks the deal as completed, and updates the campaign and influencer buckets
	if err := srv.db.Update(func(tx *bolt.Tx) (err error) {
		var (
			cmp *common.Campaign
		)

		err = json.Unmarshal(tx.Bucket([]byte(srv.Cfg.Bucket.Campaign)).Get([]byte(d.CampaignId)), &cmp)
		if err != nil {
			log.Println("Error unmarshallign campaign", err)
			return err
		}

		if !cmp.IsValid() {
			return errors.New("Campaign is no longer active")
		}

		d.Completed = int32(time.Now().Unix())
		cmp.Deals[d.Id] = d

		inf := srv.auth.GetInfluencerTx(tx, d.InfluencerId)
		if inf == nil {
			log.Println("Error unmarshalling influencer")
			return ErrUnmarshal
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
		if err := saveInfluencer(srv, tx, inf); err != nil {
			log.Println("Error saving influencer!", err)
			return err
		}

		// Save the campaign!
		if err := saveCampaign(tx, cmp, srv); err != nil {
			log.Println("Error saving campaign!", err)
			return err
		}

		return nil
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

func hasReqHash(text string, hashtags []string) bool {
	if len(hashtags) > 0 {
		for _, tg := range hashtags {
			for _, reqHash := range requiredHash {
				if strings.EqualFold(tg, reqHash) {
					return true
				}
			}
		}
	}

	if len(text) > 0 {
		for _, reqHash := range requiredHash {
			if containsFold(text, reqHash) {
				return true
			}
		}
	}

	return false
}

func findTwitterMatch(inf *auth.Influencer, deal *common.Deal) *twitter.Tweet {
	if inf.Twitter == nil {
		return nil
	}

	for _, tw := range inf.Twitter.LatestTweets {
		if int32(tw.CreatedAt.Unix()) < deal.Assigned {
			continue
		}

		postTags := tw.Hashtags()
		// check for required hashes!
		if !hasReqHash(tw.Text, postTags) {
			continue
		}

		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if strings.EqualFold(hashtag, tg) {
						foundHash = true
					}
				}
				if containsFold(tw.Text, tg) {
					foundHash = true
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
				if strings.EqualFold(mt, deal.Mention) {
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
				if strings.EqualFold(l, deal.Link) {
					foundLink = true
				}
			}
			if containsFold(tw.Text, deal.Link) {
				foundLink = true
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

func findFacebookMatch(inf *auth.Influencer, deal *common.Deal) *facebook.Post {
	if inf.Facebook == nil {
		return nil
	}

	for _, post := range inf.Facebook.LatestPosts {
		if int32(post.Published.Unix()) < deal.Assigned {
			continue
		}

		postTags := post.Hashtags()
		if !hasReqHash(post.Caption, postTags) {
			continue
		}

		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if strings.EqualFold(hashtag, tg) {
						foundHash = true
					}
				}
				if containsFold(post.Caption, tg) {
					foundHash = true
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if containsFold(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if containsFold(post.Caption, deal.Link) {
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

func findInstagramMatch(inf *auth.Influencer, deal *common.Deal) *instagram.Post {
	if inf.Instagram == nil {
		return nil
	}

	for _, post := range inf.Instagram.LatestPosts {
		if post.Published < deal.Assigned {
			continue
		}

		if !hasReqHash(post.Caption, post.Hashtags) {
			continue
		}

		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			for _, tg := range deal.Tags {
				for _, hashtag := range post.Hashtags {
					if strings.EqualFold(hashtag, tg) {
						foundHash = true
					}
				}
				if containsFold(post.Caption, tg) {
					foundHash = true
				}
			}

			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if containsFold(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if containsFold(deal.Link, inf.Instagram.LinkInBio) || strings.Contains(post.Caption, deal.Link) {
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

func findYouTubeMatch(inf *auth.Influencer, deal *common.Deal) *youtube.Post {
	if inf.YouTube == nil {
		return nil
	}

	for _, post := range inf.YouTube.LatestPosts {
		if post.Published < deal.Assigned {
			continue
		}

		postTags := post.Hashtags()
		if !hasReqHash(post.Description, postTags) {
			continue
		}

		var foundHash, foundMention, foundLink bool
		if len(deal.Tags) > 0 {
			for _, tg := range deal.Tags {
				for _, hashtag := range postTags {
					if containsFold(strings.ToLower(hashtag), tg) {
						foundHash = true
					}
				}

				if containsFold(post.Description, tg) {
					foundHash = true
				}
			}
			if !foundHash {
				continue
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			if containsFold(post.Description, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			}
		} else {
			foundMention = true
		}

		if deal.Link != "" {
			if containsFold(post.Description, deal.Link) {
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

func containsFold(haystack, needle string) bool {
	haystack = strings.TrimSpace(haystack)
	needle = strings.TrimSpace(needle)
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
