package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

var (
	requiredHash = []string{"ad", "promotion", "sponsored", "sponsoredPost", "paidPost", "endorsement", "endorsed", "advertisement", "ads"}
)

const (
	timeoutDays    = 25
	timeoutSeconds = int32(60*60*24) * timeoutDays
	waitingPeriod  = int32(16) // Wait 16 hours before we accept a deal
	minRatio       = 0.04      // Minimum comments to like ratio as a percentage
)

func explore(srv *Server) (int32, error) {
	var (
		foundDeals int32
	)

	// Traverses active deals in our system and checks
	// to see whether they have been satisfied or have timed out
	activeDeals, err := common.GetAllActiveDeals(srv.db, srv.Cfg)
	if err != nil {
		return 0, err
	}

	// The influencer has 14 days to do the deal before it's put
	// back into the pool
	now := int32(time.Now().Unix())
	minTs := now - (timeoutSeconds)

	for _, deal := range activeDeals {
		// Lets check to make sure the subscription is active before
		// we start approving deals!
		adv := srv.auth.GetAdvertiser(deal.AdvertiserId)
		if adv == nil {
			srv.Notify("Couldn't find advertiser "+deal.AdvertiserId, "Check ASAP!")
			continue
		}

		allowed, err := subscriptions.IsSubscriptionActive(adv.IsSelfServe(), adv.Subscription)
		if err != nil {
			srv.Alert("Stripe subscription lookup error for "+adv.Subscription, err)
			continue
		}

		if !allowed {
			continue
		}

		var foundPost bool

		// Go over all assigned deals in the platform
		inf, ok := srv.auth.Influencers.Get(deal.InfluencerId)
		if !ok {
			log.Println("Failed to unmarshal influencer!")
			continue
		}

		if deal.Perk != nil && !deal.Perk.Status {
			// Perk hasn't been sent!
			continue
		}

		if inf.IsBanned() {
			// This foo got banned lets clear the deal!
			if err := clearDeal(srv, deal.Id, deal.InfluencerId, deal.CampaignId, true); err != nil {
				log.Println("Error clearing deal!", deal.Id, err)
				continue
			}

			if err := srv.Cfg.Loggers.Log("deals", map[string]interface{}{
				"action": "banned cleared deal",
				"deal":   deal,
			}); err != nil {
				log.Println("Failed to log banned cleared deal!", inf.Id, deal.CampaignId)
			}

			continue
		}

		targetLink := trimURLPrefix(deal.ShortenedLink)

		for _, mediaPlatform := range deal.Platforms {
			if foundPost {
				break
			}

			// Iterate over all the available platforms and
			// assign the first one that matches
			switch mediaPlatform {
			case platform.YouTube:
				if post := findYouTubeMatch(srv, inf, deal, targetLink); post != nil {
					if err = srv.ApproveYouTube(post, deal); err != nil {
						msg := fmt.Sprintf("Failed to approve YT post for %s", inf.Id)
						srv.Alert(msg, err)
						continue
					}
					foundPost = true
					foundDeals += 1
					if err = inf.DealCompletion(deal, srv.Cfg); err != nil {
						srv.Alert("Failed to alert influencer of completion: "+inf.Id, err)
					}
					break
				}
			case platform.Instagram:
				if post := findInstagramMatch(srv, inf, deal, targetLink); post != nil {
					if err = srv.ApproveInstagram(post, deal); err != nil {
						msg := fmt.Sprintf("Failed to approve insta post for %s", inf.Id)
						srv.Alert(msg, err)
						continue
					}
					foundPost = true
					foundDeals += 1
					if err = inf.DealCompletion(deal, srv.Cfg); err != nil {
						srv.Alert("Failed to alert influencer of completion: "+inf.Id, err)
					}
					break
				}
			case platform.Twitter:
				if tweet := findTwitterMatch(srv, inf, deal, targetLink); tweet != nil {
					if err = srv.ApproveTweet(tweet, deal); err != nil {
						msg := fmt.Sprintf("Failed to approve tweet for %s", inf.Id)
						srv.Alert(msg, err)
						continue
					}
					foundPost = true
					foundDeals += 1
					if err = inf.DealCompletion(deal, srv.Cfg); err != nil {
						srv.Alert("Failed to alert influencer of completion: "+inf.Id, err)
					}
					break
				}
			case platform.Facebook:
				if post := findFacebookMatch(srv, inf, deal, targetLink); post != nil {
					if err = srv.ApproveFacebook(post, deal); err != nil {
						msg := fmt.Sprintf("Failed to approve fb post for %s", inf.Id)
						srv.Alert(msg, err)
						continue
					}
					foundPost = true
					foundDeals += 1
					if err = inf.DealCompletion(deal, srv.Cfg); err != nil {
						srv.Alert("Failed to alert influencer of completion: "+inf.Id, err)
					}
					break
				}
			default:
				continue
			}
		}

		// If the deal has not been approved and it has gone past the
		// dealTimeout.. put it back in the pool!

		// Lets exclude timeouts for signal for now
		if deal.Completed == 0 && !deal.PickedUp {
			hoursSinceAssigned := (now - deal.Assigned) / 3600
			if hoursSinceAssigned > 24*(timeoutDays-7) && hoursSinceAssigned <= (24*(timeoutDays-7))+EngineRunTime {
				// Lets warn the influencer that they have 7 days left!
				// NOTE.. the engine run time offset is so that it only runs once per engine
				// run
				if err := inf.DealHeadsUp(deal, srv.Cfg); err != nil {
					srv.Alert(fmt.Sprintf("Error emailing deal heads up to %s for deal %s", inf.Id, deal.Id), err)
				}
			} else if minTs > deal.Assigned {
				// Ok lets time out UNLESS the deal has been picked up and is waiting admin approval!
				if err := clearDeal(srv, deal.Id, deal.InfluencerId, deal.CampaignId, true); err != nil {
					return foundDeals, err
				}
				if err := srv.Cfg.Loggers.Log("deals", map[string]interface{}{
					"action": "timeout",
					"deal":   deal,
				}); err != nil {
					log.Println("Failed to log cleared deal!", inf.Id, deal.CampaignId)
				}

				if err := inf.DealTimeout(deal, srv.Cfg); err != nil {
					srv.Alert(fmt.Sprintf("Error emailing timeout emails  to %s for deal %s", inf.Id, deal.Id), err)
				}
			}
		}
	}
	return foundDeals, nil
}

var urlErr = errors.New("Failed to retrieve post URL")

func (srv *Server) CompleteDeal(d *common.Deal, completion int32) error {
	if d.PostUrl == "" {
		return urlErr
	}

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

		// Lets delete all clicks that were done before completion
		if completion == 0 {
			completion = d.Completed
		}
		d.Reporting = d.SanitizeClicks(completion)

		cmp.Deals[d.Id] = d

		inf, ok := srv.auth.Influencers.Get(d.InfluencerId)
		if !ok {
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

		// Lets add to timeline!
		cmp.AddToTimeline(common.CAMPAIGN_SUCCESS, true, srv.Cfg)

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

	if err := srv.Cfg.Loggers.Log("deals", map[string]interface{}{
		"action": "approved",
		"deal":   d,
	}); err != nil {
		log.Println("Failed to log appproved deal!", d.InfluencerId, d.CampaignId)
	}

	return nil
}

func (srv *Server) ApproveTweet(tweet *twitter.Tweet, d *common.Deal) error {
	d.Tweet = tweet
	d.PostUrl = tweet.PostURL
	d.AssignedPlatform = platform.Twitter
	return srv.CompleteDeal(d, int32(tweet.CreatedAt.Unix()))
}

func (srv *Server) ApproveFacebook(post *facebook.Post, d *common.Deal) error {
	d.Facebook = post
	d.PostUrl = post.PostURL
	d.AssignedPlatform = platform.Facebook
	return srv.CompleteDeal(d, int32(post.Published.Unix()))
}

func (srv *Server) ApproveInstagram(post *instagram.Post, d *common.Deal) error {
	d.Instagram = post
	d.PostUrl = post.PostURL
	d.AssignedPlatform = platform.Instagram
	return srv.CompleteDeal(d, post.Published)
}

func (srv *Server) ApproveYouTube(post *youtube.Post, d *common.Deal) error {
	d.YouTube = post
	d.PostUrl = post.PostURL
	d.AssignedPlatform = platform.YouTube
	return srv.CompleteDeal(d, post.Published)
}

func hasReqHash(text string, hashtags []string) bool {
	for _, tg := range hashtags {
		for _, reqHash := range requiredHash {
			if strings.EqualFold(tg, reqHash) {
				return true
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

func findTwitterMatch(srv *Server, inf influencer.Influencer, deal *common.Deal, link string) *twitter.Tweet {
	if inf.Twitter == nil {
		return nil
	}

	for _, tw := range inf.Twitter.LatestTweets {
		postTags := tw.Hashtags()

		var (
			foundHash, foundMention, foundLink bool
			approvedFacets, consideredFacets   float64
		)

		if len(deal.Tags) > 0 {
			consideredFacets += 1
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
			} else {
				approvedFacets += 1
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			consideredFacets += 1
			for _, mt := range tw.Mentions() {
				if strings.EqualFold(mt, deal.Mention) {
					foundMention = true
				}
			}

			if !foundMention {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundMention = true
		}

		if link != "" {
			consideredFacets += 1
			for _, l := range tw.Urls() {
				if containsFold(l, link) || containsFold(link, l) {
					foundLink = true
				}
			}
			if containsFold(tw.Text, link) {
				foundLink = true
			}

			if !foundLink {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			// check for required hashes!
			if !hasReqHash(tw.Text, postTags) {
				if err := inf.DealRejection("hashtags (#ad)", tw.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
				continue
			}

			if !deal.SkipFraud {
				// If we're not skipping fraud yet we need to wait for X hours
				// before picking up the deal so we can do fraud engagement checks
				if misc.WithinLast(int32(tw.CreatedAt.Unix()), waitingPeriod) {
					// Tell the user that we have picked up their deal but waiting for admin
					// approval aka fraud check
					if err := pickupDeal(deal, inf, srv); err != nil {
						log.Println("Error emailing deal was picked up to influencer", err)
					}
					return nil
				}

				// Before returning the post.. lets check for some fraud
				var fraud []string

				// Does it have any fraud hashtags?
				for _, tg := range hashBlacklist {
					var fraudHashFound bool
					for _, hashtag := range postTags {
						if strings.EqualFold(hashtag, tg) {
							fraudHashFound = true
						}
					}
					if containsFold(tw.Text, tg) {
						fraudHashFound = true
					}

					if fraudHashFound {
						fraud = append(fraud, fmt.Sprintf("fraudulent hashtag (%s)", tg))
					}
				}

				// Lets check if engagements are WAY different than their average!
				if inf.IsViral(tw, nil, nil, nil) {
					fraud = append(fraud, "Engagements 30 percent higher than average!")
				}

				if len(fraud) > 0 {
					srv.Fraud(deal.CampaignId, deal.InfluencerId, tw.PostURL, fraud)
					return nil
				}
			}

			return tw
		} else {
			if consideredFacets > 1 && approvedFacets/consideredFacets >= 0.5 {
				// If we got more than 50% of the facets approved but didn't pass..
				// lets notify the influencer!
				var reason string
				if !foundHash {
					reason = "hashtags"
				} else if !foundLink {
					reason = "required link"
				} else if !foundMention {
					reason = "required mention"
				}
				if err := inf.DealRejection(reason, tw.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
			}
		}
	}

	return nil
}

func findFacebookMatch(srv *Server, inf influencer.Influencer, deal *common.Deal, link string) *facebook.Post {
	if inf.Facebook == nil {
		return nil
	}

	for _, post := range inf.Facebook.LatestPosts {
		postTags := post.Hashtags()

		var (
			foundHash, foundMention, foundLink bool
			approvedFacets, consideredFacets   float64
		)

		if len(deal.Tags) > 0 {
			consideredFacets += 1
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
			} else {
				approvedFacets += 1
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			consideredFacets += 1
			if containsFold(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundMention = true
		}

		if link != "" {
			consideredFacets += 1
			if containsFold(post.Caption, link) {
				foundLink = true
			}

			if !foundLink {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			if !hasReqHash(post.Caption, postTags) {
				if err := inf.DealRejection("hashtags (#ad)", post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
				continue
			}

			if !deal.SkipFraud {
				if misc.WithinLast(int32(post.Published.Unix()), waitingPeriod) {
					if err := pickupDeal(deal, inf, srv); err != nil {
						log.Println("Error emailing deal was picked up to influencer", err)
					}
					return nil
				}

				// Before returning the post.. lets check for some fraud
				var fraud []string

				// Does it have any fraud hashtags?
				for _, tg := range hashBlacklist {
					var fraudHashFound bool
					for _, hashtag := range postTags {
						if strings.EqualFold(hashtag, tg) {
							fraudHashFound = true
						}
					}
					if containsFold(post.Caption, tg) {
						fraudHashFound = true
					}

					if fraudHashFound {
						fraud = append(fraud, fmt.Sprintf("fraudulent hashtag (%s)", tg))
					}
				}

				// What's the likes to comments ratio?
				if checkRatio(post.Likes, post.Comments) {
					fraud = append(fraud, "Likes to comments ratio is suspicious")
				}

				// Lets check if engagements are WAY different than their average!
				if inf.IsViral(nil, nil, post, nil) {
					fraud = append(fraud, "Engagements 30 percent higher than average!")
				}

				if len(fraud) > 0 {
					srv.Fraud(deal.CampaignId, deal.InfluencerId, post.PostURL, fraud)
					return nil
				}
			}

			return post
		} else {
			if consideredFacets > 1 && approvedFacets/consideredFacets >= 0.5 {
				// If we got more than 50% of the facets approved but didn't pass..
				// lets notify the influencer!
				var reason string
				if !foundHash {
					reason = "hashtags"
				} else if !foundLink {
					reason = "link"
				} else if !foundMention {
					reason = "mention"
				}
				if err := inf.DealRejection(reason, post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
			}
		}
	}

	return nil
}

func findInstagramMatch(srv *Server, inf influencer.Influencer, deal *common.Deal, link string) *instagram.Post {
	if inf.Instagram == nil {
		return nil
	}

	rejections := make(map[string]string)

	for _, post := range inf.Instagram.LatestPosts {
		var (
			foundHash, foundMention, foundLink bool
			approvedFacets, consideredFacets   float64
		)

		if len(deal.Tags) > 0 {
			consideredFacets += 1
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
				rejections[post.Caption] = "NO_HASH"
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			consideredFacets += 1
			if containsFold(post.Caption, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				rejections[post.Caption] = "NO_MENTION"
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundMention = true
		}

		if link != "" {
			consideredFacets += 1
			if containsFold(inf.Instagram.LinkInBio, link) || containsFold(link, inf.Instagram.LinkInBio) || strings.Contains(post.Caption, link) {
				foundLink = true
			}

			if !foundLink {
				rejections[post.Caption] = "NO_LINK"
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			if !hasReqHash(post.Caption, post.Hashtags) {
				if err := inf.DealRejection("hashtags (#ad)", post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
				continue
			}

			if !deal.SkipFraud {
				if misc.WithinLast(int32(post.Published), waitingPeriod) {
					rejections[post.Caption] = "WAITING_PERIOD"
					if err := pickupDeal(deal, inf, srv); err != nil {
						log.Println("Error emailing deal was picked up to influencer", err)
					}
					return nil
				}

				// Before returning the post.. lets check for some fraud
				var fraud []string

				// Does it have any fraud hashtags?
				for _, tg := range hashBlacklist {
					var fraudHashFound bool

					for _, hashtag := range post.Hashtags {
						if strings.EqualFold(hashtag, tg) {
							fraudHashFound = true
						}
					}

					if containsFold(post.Caption, tg) {
						fraudHashFound = true
					}

					if fraudHashFound {
						fraud = append(fraud, fmt.Sprintf("fraudulent hashtag (%s)", tg))
					}
				}

				// What's the likes to comments ratio?
				if checkRatio(post.Likes, post.Comments) {
					fraud = append(fraud, "Likes to comments ratio is suspicious")
				}

				// Lets check if engagements are WAY different than their average!
				if inf.IsViral(nil, post, nil, nil) {
					fraud = append(fraud, "Engagements 30 percent higher than average!")
				}

				if len(fraud) > 0 {
					srv.Fraud(deal.CampaignId, deal.InfluencerId, post.PostURL, fraud)
					return nil
				}
			}

			return post
		} else {
			if consideredFacets > 1 && approvedFacets/consideredFacets >= 0.5 {
				// If we got more than 50% of the facets approved but didn't pass..
				// lets notify the influencer!
				var reason string
				if !foundHash {
					reason = "hashtags"
				} else if !foundLink {
					reason = "link"
				} else if !foundMention {
					reason = "mention"
				}
				if err := inf.DealRejection(reason, post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
			}
		}
	}

	return nil
}

func findYouTubeMatch(srv *Server, inf influencer.Influencer, deal *common.Deal, link string) *youtube.Post {
	if inf.YouTube == nil {
		return nil
	}

	for _, post := range inf.YouTube.LatestPosts {
		postTags := post.Hashtags()

		var (
			foundHash, foundMention, foundLink bool
			approvedFacets, consideredFacets   float64
		)
		if len(deal.Tags) > 0 {
			consideredFacets += 1
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
			} else {
				approvedFacets += 1
			}
		} else {
			foundHash = true
		}

		if deal.Mention != "" {
			consideredFacets += 1
			if containsFold(post.Description, deal.Mention) {
				foundMention = true
			}

			if !foundMention {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundMention = true
		}

		if link != "" {
			consideredFacets += 1
			if containsFold(post.Description, link) {
				foundLink = true
			}

			if !foundLink {
				continue
			} else {
				approvedFacets += 1
			}
		} else {
			foundLink = true
		}

		if foundHash && foundMention && foundLink {
			if !hasReqHash(post.Description, postTags) {
				if err := inf.DealRejection("hashtags (#ad)", post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
				continue
			}

			if !deal.SkipFraud {
				if misc.WithinLast(post.Published, waitingPeriod) {
					if err := pickupDeal(deal, inf, srv); err != nil {
						log.Println("Error emailing deal was picked up to influencer", err)
					}
					return nil
				}

				// Before returning the post.. lets check for some fraud
				var fraud []string

				// Does it have any fraud hashtags?
				for _, tg := range hashBlacklist {
					var fraudHashFound bool
					for _, hashtag := range postTags {
						if strings.EqualFold(hashtag, tg) {
							fraudHashFound = true
						}
					}
					if containsFold(post.Description, tg) {
						fraudHashFound = true
					}

					if fraudHashFound {
						fraud = append(fraud, fmt.Sprintf("fraudulent hashtag (%s)", tg))
					}
				}

				// What's the likes to comments ratio?
				if checkRatio(post.Likes, post.Comments) {
					fraud = append(fraud, "Likes to comments ratio is suspicious")
				}

				// Lets check if engagements are WAY different than their average!
				if inf.IsViral(nil, nil, nil, post) {
					fraud = append(fraud, "Engagements 30 percent higher than average!")
				}

				if len(fraud) > 0 {
					srv.Fraud(deal.CampaignId, deal.InfluencerId, post.PostURL, fraud)
					return nil
				}
			}

			return post
		} else {
			if consideredFacets > 1 && approvedFacets/consideredFacets >= 0.5 {
				// If we got more than 50% of the facets approved but didn't pass..
				// lets notify the influencer!
				var reason string
				if !foundHash {
					reason = "hashtags"
				} else if !foundLink {
					reason = "link"
				} else if !foundMention {
					reason = "mention"
				}
				if err := inf.DealRejection(reason, post.PostURL, deal, srv.Cfg); err != nil {
					log.Println("Error emailing rejection reason to influencer", err)
				}
			}
		}
	}

	return nil
}

func containsFold(haystack, needle string) bool {
	haystack = strings.TrimSpace(haystack)
	needle = strings.TrimSpace(needle)
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func checkRatio(likes, comments float64) bool {
	return comments/likes > minRatio
}

func pickupDeal(deal *common.Deal, inf influencer.Influencer, srv *Server) error {
	if deal.PickedUp {
		return nil
	}

	if err := inf.DealPickedUp(deal, srv.Cfg); err != nil {
		return err
	}

	deal.PickedUp = true

	for _, infDeal := range inf.ActiveDeals {
		if deal.Id == infDeal.Id {
			deal.PickedUp = true
			break
		}
	}

	return saveAllActiveDeals(srv, inf)
}
