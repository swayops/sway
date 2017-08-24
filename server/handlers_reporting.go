package server

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

func getCampaignReport(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.Param("cid")
		if cid == "" {
			misc.WriteJSON(c, 500, misc.StatusErr("Please pass in a valid campaign ID"))
			return
		}

		from := reporting.GetReportDate(c.Param("from"))
		to := reporting.GetReportDate(c.Param("to"))
		if from.IsZero() || to.IsZero() || to.Before(from) {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		isPDF, _ := strconv.ParseBool(c.Query("pdf"))
		if err := reporting.GenerateCampaignReport(c, s.auth, s.db, cid, from, to, isPDF, s.Cfg); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
		}
	}
}

type AdminStats struct {
	AdAgencies  int `json:"adAgencies"`  // Total # of Ad Agencies
	Advertisers int `json:"advertisers"` // Total # of Advertisers
	Campaigns   int `json:"cmps"`        // Total # of Campaigns

	PerksInbound   int `json:"perkInb"`     // Total # of Perks Inbound
	PerksStored    int `json:"perkStore"`   // Total # of Perks Stored
	PerksOutbound  int `json:"perkOut"`     // Total # of Perks Outbound
	PerksDelivered int `json:"perkDeliver"` // Total # of Perks Delivered

	DealsAccepted  int     `json:"dealAccepted"`   // Total # of Deals Accepted
	DealsCompleted int     `json:"dealCompleted"`  // Total # of Deals Completed
	CompletionRate float64 `json:"completionRate"` // Percentage of deals completed

	TalentAgencies    int     `json:"talentAgencies"`    // Total # of Talent Agencies
	InfPerTalent      int     `json:"infPerTalent"`      // # of Influencers per Talent Agency
	TotalAgencyPayout float64 `json:"totalAgencyPayout"` // Total $ paid out to Talent Agencies

	Influencers           int     `json:"influencers"`           // Total # of Influencers
	TotalInfluencerPayout float64 `json:"totalInfluencerPayout"` // Total $ paid out to Influencers
	Reach                 int64   `json:"reach"`                 // Total influencer reach
	Likes                 int32   `json:"likes"`                 // Total # of Likes generated by deal posts
	Comments              int32   `json:"comments"`              // Total # of Comments generated by deal posts
	Shares                int32   `json:"shares"`                // Total # of Shares generated by deal posts
	Views                 int32   `json:"views"`                 // Total # of Views generated by deal posts
	Clicks                int32   `json:"clicks"`                // Total # of Clicks generated by deal posts
}

func getAdminStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			perksInbound, perksStored, perksOutbound, perksDelivered, dealsAccept, dealsComplete int
			a                                                                                    *AdminStats
		)

		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}

				if cmp.Approved == 0 {
					// This is a campaign who's perks we are waiting for! (inbound)
					if cmp.Perks != nil {
						perksInbound += cmp.Perks.Count
					}
				} else {
					// This is a campaign that's been approved (we have all their perks)
					if cmp.Perks != nil {
						perksStored += cmp.Perks.Count
					}
				}

				if !cmp.IsValid() {
					return nil
				}

				for _, d := range cmp.Deals {
					if d.Perk != nil && d.InfluencerId != "" {
						if !d.Perk.Status {
							// This deal has been picked up.. there's a perk attached
							// and the status is false (meaning it hasn't been mailed yet)
							perksOutbound += 1
						} else {
							// This deal is set to true meaning its been mailed!
							perksDelivered += 1
						}
					}

					if d.IsActive() {
						dealsAccept += 1
					}

					if d.IsComplete() {
						dealsComplete += 1
					}
				}
				return
			})

			talentAgencyCount := len(getTalentAgencies(s, tx))
			var (
				infCount                               int
				reach                                  int64
				likes, comments, shares, views, clicks int32
				totalInfluencer, totalAgency           float64
			)

			for _, inf := range s.auth.Influencers.GetAll() {
				reach += inf.GetFollowers()
				infCount += 1
				for _, d := range inf.CompletedDeals {
					stats := d.TotalStats()
					totalInfluencer += stats.Influencer
					totalAgency += stats.Agency
					likes += stats.Likes
					comments += stats.Comments
					shares += stats.Shares
					views += stats.Views
					clicks += stats.GetClicks()
				}
			}

			var completionRate float64
			if dealsComplete > 0 {
				completionRate = 100 * (float64(dealsComplete) / float64(dealsComplete+dealsAccept))
			}

			a = &AdminStats{
				AdAgencies:            len(getAdAgencies(s, tx)),
				Advertisers:           len(getAdvertisers(s, tx)),
				Campaigns:             s.Campaigns.Len(),
				PerksInbound:          perksInbound,
				PerksStored:           perksStored,
				PerksOutbound:         perksOutbound,
				PerksDelivered:        perksDelivered,
				DealsAccepted:         dealsAccept,
				DealsCompleted:        dealsComplete,
				CompletionRate:        completionRate,
				TalentAgencies:        talentAgencyCount,
				Influencers:           infCount,
				InfPerTalent:          int(float32(infCount) / float32(talentAgencyCount)),
				TotalAgencyPayout:     totalAgency,
				TotalInfluencerPayout: totalInfluencer,
				Reach:    reach,
				Likes:    likes,
				Comments: comments,
				Shares:   shares,
				Views:    views,
				Clicks:   clicks,
			}

			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, a)
	}
}

func getAdvertiserTimeline(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			targetAdv = c.Param("id")
		)

		cmpTimeline := make(map[string]*common.Timeline)
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv && len(cmp.Timeline) > 0 && !cmp.Archived {
					cmpTimeline[fmt.Sprintf("%s (%s)", cmp.Name, cmp.Id)] = cmp.Timeline[len(cmp.Timeline)-1]
				}
				return
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		common.SetLinkTitles(cmpTimeline)
		misc.WriteJSON(c, 200, cmpTimeline)
	}
}

func getAdvertiserStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			start, _  = strconv.Atoi(c.Param("start"))
			end, _    = strconv.Atoi(c.Param("end"))
			targetAdv = c.Param("id")
			campaigns []*common.Campaign
			cmpStats  []map[string]*reporting.Totals
		)

		if start == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
					campaigns = append(campaigns, &cmp)
				}
				return
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		if targetAdv == "81" {
			// Hack for demo account
			startTmp, _ := time.Parse("Jan 2, 2006", "Jan 30, 2017")
			endTmp, _ := time.Parse("Jan 2, 2006", "Mar 1, 2017")

			start = int(time.Since(startTmp) / (24 * time.Hour))
			end = int(time.Since(endTmp) / (24 * time.Hour))
		}

		for _, cmp := range campaigns {
			stats := reporting.GetCampaignBreakdown(cmp.Id, s.db, s.Cfg, start, end)
			cmpStats = append(cmpStats, stats)
		}

		misc.WriteJSON(c, 200, reporting.Merge(cmpStats))
	}
}

func getCampaignStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		misc.WriteJSON(c, 200, reporting.GetCampaignBreakdown(c.Param("cid"), s.db, s.Cfg, days, 0))
	}
}

func getInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("influencerId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		misc.WriteJSON(c, 200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, "", ""))
	}
}

func getCampaignInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("infId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		misc.WriteJSON(c, 200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, c.Param("cid"), ""))
	}
}

func getAgencyInfluencerStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, err := strconv.Atoi(c.Param("days"))
		if err != nil || days == 0 {
			misc.WriteJSON(c, 500, misc.StatusErr("Invalid date range!"))
			return
		}

		inf, ok := s.auth.Influencers.Get(c.Param("infId"))
		if !ok {
			misc.WriteJSON(c, 500, misc.StatusErr("Error retrieving influencer!"))
			return
		}

		misc.WriteJSON(c, 200, reporting.GetInfluencerBreakdown(inf, s.Cfg, days, inf.Rep, inf.CurrentRep, "", c.Param("id")))
	}
}

type FeedCell struct {
	Username     string `json:"username,omitempty"`
	InfluencerID string `json:"infID,omitempty"`
	URL          string `json:"url,omitempty"`
	Caption      string `json:"caption,omitempty"`

	CampaignID   string `json:"campaignID,omitempty"`
	CampaignName string `json:"campaignName,omitempty"`

	Published int32 `json:"published,omitempty"`

	Views       int32 `json:"views,omitempty"`
	Likes       int32 `json:"likes,omitempty"`
	Clicks      int32 `json:"clicks,omitempty"`
	Uniques     int32 `json:"uniques,omitempty"`
	Conversions int32 `json:"conversions,omitempty"`
	Comments    int32 `json:"comments,omitempty"`
	Shares      int32 `json:"shares,omitempty"`

	Bonus bool `json:"bonus,omitempty"`

	// Used by dash to display proper image
	SocialImage string `json:"socialImage,omitempty"`

	ProfilePicture string `json:"profilePicture,omitempty"`
	PostPicture    string `json:"postPicture,omitempty"`
}

func (d *FeedCell) UseTweet(tw *twitter.Tweet, profile *twitter.Twitter) {
	d.Caption = tw.Text
	d.Published = int32(tw.CreatedAt.Unix())
	d.URL = tw.PostURL
	d.SocialImage = profile.ProfilePicture

	d.ProfilePicture = profile.ProfilePicture
}

func (d *FeedCell) UseInsta(insta *instagram.Post, profile *instagram.Instagram) {
	d.Caption = insta.Caption
	d.Published = insta.Published
	d.URL = insta.PostURL
	if insta.Thumbnail != "" && misc.Ping(insta.Thumbnail) == nil {
		d.SocialImage = insta.Thumbnail
		d.PostPicture = insta.Thumbnail
	} else {
		d.SocialImage = profile.ProfilePicture
	}

	d.ProfilePicture = profile.ProfilePicture
}

func (d *FeedCell) UseFB(fb *facebook.Post, profile *facebook.Facebook) {
	d.Caption = fb.Caption
	d.Published = int32(fb.Published.Unix())
	d.URL = fb.PostURL
	d.SocialImage = profile.ProfilePicture

	d.ProfilePicture = profile.ProfilePicture
}

func (d *FeedCell) UseYT(yt *youtube.Post, profile *youtube.YouTube) {
	d.Caption = yt.Description
	d.Published = yt.Published
	d.URL = yt.PostURL
	if yt.Thumbnail != "" && misc.Ping(yt.Thumbnail) == nil {
		d.SocialImage = yt.Thumbnail
		d.PostPicture = yt.Thumbnail
	} else {
		d.SocialImage = profile.ProfilePicture
	}

	d.ProfilePicture = profile.ProfilePicture
}

func getAdvertiserContentFeed(s *Server, requireKey bool) gin.HandlerFunc {
	// Retrieves all completed deals by advertiser
	return func(c *gin.Context) {
		if requireKey && c.Query("key") != "7d7e8c4486c8" {
			misc.WriteJSON(c, 401, misc.StatusErr("Unauthorized"))
			return
		}

		adv := s.auth.GetAdvertiser(c.Param("id"))
		if adv == nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		var feed []FeedCell
		if err := s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp common.Campaign
				if err := json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if cmp.AdvertiserId == adv.ID {
					for _, deal := range cmp.Deals {
						if deal.Completed > 0 {
							d := FeedCell{
								CampaignID:   cmp.Id,
								CampaignName: cmp.Name,
								Username:     deal.InfluencerName,
								InfluencerID: deal.InfluencerId,
							}

							total := deal.TotalStats()
							d.Likes = total.Likes
							d.Comments = total.Comments
							d.Shares = total.Shares
							d.Views = total.Views
							d.Clicks = total.GetClicks()
							d.Uniques = total.GetUniqueClicks()
							d.Conversions = int32(len(total.Conversions))

							inf, ok := s.auth.Influencers.Get(deal.InfluencerId)
							if !ok {
								log.Println("Influencer not found!", deal.InfluencerId)
								continue
							}

							if deal.Tweet != nil {
								d.UseTweet(deal.Tweet, inf.Twitter)
							} else if deal.Facebook != nil {
								d.UseFB(deal.Facebook, inf.Facebook)
							} else if deal.Instagram != nil {
								d.UseInsta(deal.Instagram, inf.Instagram)
							} else if deal.YouTube != nil {
								d.UseYT(deal.YouTube, inf.YouTube)
							}

							feed = append(feed, d)

							// Lets add extra cells for any bonus posts
							if deal.Bonus != nil {
								d.Bonus = true
								// Lets copy the cell so we can re-use values!
								for _, tw := range deal.Bonus.Tweet {
									dupeCell := d
									dupeCell.UseTweet(tw, inf.Twitter)
									dupeCell.Likes = int32(tw.Favorites)
									dupeCell.Comments = 0
									dupeCell.Shares = int32(tw.Retweets)
									dupeCell.Clicks = 0
									dupeCell.Views = common.GetViews(dupeCell.Likes, dupeCell.Comments, dupeCell.Shares)

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.Facebook {
									dupeCell := d
									dupeCell.UseFB(post, inf.Facebook)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = int32(post.Shares)
									dupeCell.Clicks = 0
									dupeCell.Views = common.GetViews(dupeCell.Likes, dupeCell.Comments, dupeCell.Shares)

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.Instagram {
									dupeCell := d
									dupeCell.UseInsta(post, inf.Instagram)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = 0
									dupeCell.Clicks = 0
									dupeCell.Views = common.GetViews(dupeCell.Likes, dupeCell.Comments, dupeCell.Shares)

									feed = append(feed, dupeCell)
								}

								for _, post := range deal.Bonus.YouTube {
									dupeCell := d
									dupeCell.UseYT(post, inf.YouTube)

									dupeCell.Likes = int32(post.Likes)
									dupeCell.Comments = int32(post.Comments)
									dupeCell.Shares = 0
									dupeCell.Views = int32(post.Views)
									dupeCell.Clicks = 0

									feed = append(feed, dupeCell)
								}
							}
						}
					}
				}
				return
			})
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr("Internal error"))
			return
		}

		misc.WriteJSON(c, 200, feed)
	}
}

func syncAllStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, inf := range s.auth.Influencers.GetAll() {
			for _, deal := range inf.CompletedDeals {
				if deal.IsComplete() {
					// Lets make sure numbers for likes and comments on insta
					// post line up with daily stats
					if deal.Instagram != nil {
						totalLikes := int32(deal.Instagram.Likes)
						totalComments := int32(deal.Instagram.Comments)

						var (
							reportingLikes, reportingComments int32
							key                               string
							stats                             *common.Stats
							highestLikes                      int32
						)

						for day, target := range deal.Reporting {
							if target.Likes > highestLikes {
								highestLikes = target.Likes

								stats = target
								key = day
							}
							reportingLikes += target.Likes
							reportingComments += target.Comments
						}

						if stats == nil || key == "" {
							continue
						}

						likesDiff := reportingLikes - totalLikes
						if likesDiff > 0 {
							// Subtract likes from stats

							if stats.Likes > likesDiff {
								// We have all the likes we need on 31st.. lets surtact!
								log.Println("Need to take out likes:", deal.Id, likesDiff)
								stats.Likes -= likesDiff
							}
						} else if likesDiff < 0 {
							// meaning we need to ADD likes
							stats.Likes += totalLikes - reportingLikes
						}

						commentsDiff := reportingComments - totalComments
						if commentsDiff > 0 {
							// Subtract comments from stats

							if stats.Comments >= commentsDiff {
								log.Println("Need to take out comments:", deal.Id, commentsDiff)

								// We have all the likes we need on 31st.. lets surtact!
								stats.Comments -= commentsDiff
							} else if commentsDiff < 0 {
								stats.Comments += totalComments - reportingComments
							}
						}

						// Save and bail
						deal.Reporting[key] = stats
					}
				}
			}
			saveAllCompletedDeals(s, inf)
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

func getServerStats(s *Server) gin.HandlerFunc {
	// Returns stored server stats
	return func(c *gin.Context) {
		misc.WriteJSON(c, 200, s.Stats.Get())
	}
}
