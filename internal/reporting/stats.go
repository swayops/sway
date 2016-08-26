package reporting

import (
	"errors"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
)

// Structure of Reporting DB:
// Stores all transactions by day at the post level
// {
//     "1": { // Campaign ID
//         "2016-10-30||1||JennaMarbles||Instagram||http://www.instagram.com/post": { // Date::Name::Platform::postUrl
//             "payout": 3.4, // Includes agency fee
//             "likes": 0,
//             "comments": 0,
//             "shares": 0,
//             "views": 30,
//             "dislikes": 45,
//         },
//         "2016-10-30||2||NigaHiga||Facebook||facebook.com/post": { // Date::InfId::Platform::postUrl
//             "payout": 3.4,
//             "likes": 0,
//             "comments": 0,
//             "shares": 0,
//             "views": 30,
//             "dislikes": 45,
//         }
//     }
// }

var (
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
)

type TargetStats struct {
	Total      *Totals                 `json:"total"`
	Channel    map[string]*ReportStats `json:"channel"`
	Influencer map[string]*ReportStats `json:"influencer"`
	Post       map[string]*ReportStats `json:"post"`
}

type Totals struct {
	Influencers int32 `json:"infs,omitempty"`
	Engagements int32 `json:"engagements,omitempty"`
	Likes       int32 `json:"likes,omitempty"`
	Views       int32 `json:"views,omitempty"`
	Clicks      int32 `json:"clicks,omitempty"`

	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`

	Spent float64 `json:"spent,omitempty"`
}

type ReportStats struct {
	Likes    int32 `json:"likes,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`
	Views    int32 `json:"views,omitempty"`
	Clicks   int32 `json:"clicks,omitempty"`

	Spent       float64 `json:"spent,omitempty"`
	Rep         float64 `json:"rep,omitempty"`
	Engagements int32   `json:"engagements,omitempty"`

	AgencySpent float64 `json:"agSpent,omitempty"` // Only filled for influencer stats to get agency payment

	PlatformId   string `json:"platformId,omitempty"` // Screen name for the platform used for the deal
	Published    string `json:"posted,omitempty"`     // Pretty string of date post was made
	InfluencerId string `json:"infId,omitempty"`
	Network      string `json:"network,omitempty"` // Social Network
}

func GetCampaignStats(cid string, db *bolt.DB, cfg *config.Config, from, to time.Time, onlyTotals bool) (*TargetStats, error) {
	tg := &TargetStats{}

	// Retrieve the dates that this request requires
	dates := getDateRange(from, to)
	cmp := common.GetCampaign(cid, db, cfg)
	if cmp == nil {
		return tg, errors.New("Missing campaign!")
	}

	for _, deal := range cmp.Deals {
		if deal.Completed > 0 {
			st := deal.Get(dates)

			// This value falls in our target range!
			eng := getEngagements(st)
			views := getViews(st, eng)

			if tg.Total == nil {
				tg.Total = &Totals{}
			}

			tg.Total.Engagements += eng
			tg.Total.Likes += st.Likes
			tg.Total.Clicks += st.Clicks
			tg.Total.Views += views
			tg.Total.Spent += st.Influencer + st.TotalMarkup()
			tg.Total.Shares += st.Shares
			tg.Total.Comments += st.Comments

			// This assumes each influencer can do the deal once
			tg.Total.Influencers++

			if onlyTotals {
				continue
			}

			if tg.Channel == nil || len(tg.Channel) == 0 {
				tg.Channel = make(map[string]*ReportStats)
			}

			fillReportStats(deal.AssignedPlatform, tg.Channel, st, views, deal.InfluencerId, deal.AssignedPlatform)

			if tg.Influencer == nil || len(tg.Influencer) == 0 {
				tg.Influencer = make(map[string]*ReportStats)
			}

			fillReportStats(deal.InfluencerName, tg.Influencer, st, views, deal.InfluencerId, deal.AssignedPlatform)

			if tg.Post == nil || len(tg.Post) == 0 {
				tg.Post = make(map[string]*ReportStats)
			}

			fillContentLevelStats(deal.PostUrl, deal.AssignedPlatform, deal.Published(), tg.Post, st, views, deal.InfluencerId)

			continue
		}
	}

	if tg.Total != nil && tg.Total.Influencers == 0 {
		tg.Total.Influencers = int32(len(tg.Influencer))
	}
	return tg, nil
}

func fillReportStats(key string, data map[string]*ReportStats, st *common.Payout, views int32, infId, channel string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		stats = &ReportStats{}
		data[key] = stats
	}

	stats.Likes += st.Likes
	stats.Comments += st.Comments
	stats.Shares += st.Shares
	stats.Views += views
	stats.Clicks += st.Clicks
	stats.Spent += st.Influencer + st.TotalMarkup()
	stats.InfluencerId = infId
	stats.Network = channel
	return data
}

func fillContentLevelStats(key, platformId string, ts int32, data map[string]*ReportStats, st *common.Payout, views int32, infId string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		stats = &ReportStats{}
		data[key] = stats
	}

	stats.Likes += st.Likes
	stats.Clicks += st.Clicks
	stats.Comments += st.Comments
	stats.Shares += st.Shares
	stats.Views += views
	stats.Spent += st.Influencer + st.TotalMarkup()
	stats.PlatformId = platformId
	stats.Published = getPostDate(ts)
	stats.InfluencerId = infId

	return data
}

func GetInfluencerStats(infId string, au *auth.Auth, cfg *config.Config, from, to time.Time, cid, agid string) (*ReportStats, error) {
	stats := &ReportStats{}
	inf := au.GetInfluencer(infId)
	if inf == nil {
		return stats, errors.New("Bad influencer!")
	}
	dates := getDateRange(from, to)

	for _, deal := range inf.CompletedDeals {
		if cid != "" && deal.CampaignId != cid {
			continue
		}

		st := deal.GetByAgency(dates, agid)
		eng := getEngagements(st)
		views := getViews(st, eng)
		stats.Clicks += st.Clicks
		stats.Likes += st.Likes
		stats.Comments += st.Comments
		stats.Shares += st.Shares
		stats.Views += views
		stats.Spent += st.Influencer
		stats.AgencySpent += st.Agency
		stats.Engagements += eng
	}
	return stats, nil
}
