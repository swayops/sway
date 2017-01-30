package reporting

import (
	"errors"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

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

	Perks int32 `json:"perks,omitempty"`

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
	dates := common.GetDateRange(from, to)
	cmp := common.GetCampaign(cid, db, cfg)
	if cmp == nil {
		return tg, errors.New("Missing campaign!")
	}

	for _, deal := range cmp.Deals {
		if deal.Completed > 0 {
			st := deal.Get(dates, "")

			// This value falls in our target range!
			eng := getEngagements(st)

			if tg.Total == nil {
				tg.Total = &Totals{}
			}

			tg.Total.Engagements += eng
			tg.Total.Likes += st.Likes
			tg.Total.Clicks += st.Clicks
			tg.Total.Views += st.Views
			tg.Total.Spent += st.Influencer + st.TotalMarkup()
			tg.Total.Shares += st.Shares
			tg.Total.Comments += st.Comments
			tg.Total.Perks += st.Perks

			// This assumes each influencer can do the deal once
			if eng > 0 {
				tg.Total.Influencers++
			}

			if onlyTotals {
				continue
			}

			if tg.Channel == nil || len(tg.Channel) == 0 {
				tg.Channel = make(map[string]*ReportStats)
			}

			fillReportStats(deal.AssignedPlatform, tg.Channel, st, deal.InfluencerId, deal.AssignedPlatform)

			if tg.Influencer == nil || len(tg.Influencer) == 0 {
				tg.Influencer = make(map[string]*ReportStats)
			}

			fillReportStats(deal.InfluencerName, tg.Influencer, st, deal.InfluencerId, deal.AssignedPlatform)

			if tg.Post == nil || len(tg.Post) == 0 {
				tg.Post = make(map[string]*ReportStats)
			}

			fillContentLevelStats(deal.PostUrl, deal.AssignedPlatform, deal.Published(), tg.Post, st, deal.InfluencerId)

			continue
		}
	}

	if tg.Total != nil && tg.Total.Influencers == 0 {
		tg.Total.Influencers = int32(len(tg.Influencer))
	}
	return tg, nil
}

func fillReportStats(key string, data map[string]*ReportStats, st *common.Stats, infId, channel string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		stats = &ReportStats{}
		data[key] = stats
	}

	stats.Likes += st.Likes
	stats.Comments += st.Comments
	stats.Shares += st.Shares
	stats.Views += st.Views
	stats.Clicks += st.Clicks
	stats.Spent += st.Influencer + st.TotalMarkup()
	stats.InfluencerId = infId
	stats.Network = channel
	return data
}

func fillContentLevelStats(key, platformId string, ts int32, data map[string]*ReportStats, st *common.Stats, infId string) map[string]*ReportStats {
	stats, ok := data[key]
	if !ok {
		stats = &ReportStats{}
		data[key] = stats
	}

	stats.Likes += st.Likes
	stats.Clicks += st.Clicks
	stats.Comments += st.Comments
	stats.Shares += st.Shares
	stats.Views += st.Views
	stats.Spent += st.Influencer + st.TotalMarkup()
	stats.PlatformId = platformId
	stats.Published = getPostDate(ts)
	stats.InfluencerId = infId

	return data
}

func GetInfluencerStats(inf influencer.Influencer, cfg *config.Config, from, to time.Time, cid, agid string) (*ReportStats, error) {
	stats := &ReportStats{}
	dates := common.GetDateRange(from, to)

	for _, deal := range inf.CompletedDeals {
		if cid != "" && deal.CampaignId != cid {
			continue
		}

		st := deal.Get(dates, agid)
		eng := getEngagements(st)
		stats.Clicks += st.Clicks
		stats.Likes += st.Likes
		stats.Comments += st.Comments
		stats.Shares += st.Shares
		stats.Views += st.Views
		stats.Spent += st.Influencer
		stats.AgencySpent += st.Agency
		stats.Engagements += eng
	}
	return stats, nil
}

func GetCampaignBreakdown(cid string, db *bolt.DB, cfg *config.Config, startOffset, endOffset int) map[string]*Totals {
	// Retrieves totals for the range and campaign stats by day
	tg := make(map[string]*Totals)

	dateRange := common.GetDateRangeFromOffsetRange(startOffset, endOffset)

	// Insert totals for the range in the key "total"
	tg["total"] = &Totals{}

	// Insert day stats for the range
	for _, d := range dateRange {
		tot, err := GetCampaignStats(cid, db, cfg, d, d, true)
		if err == nil && tot != nil && tot.Total != nil {
			tg[common.GetDateFromTime(d)] = tot.Total
			val, _ := tg["total"]
			val.Clicks += tot.Total.Clicks
			val.Engagements += tot.Total.Engagements
			val.Likes += tot.Total.Likes
			val.Views += tot.Total.Views
			val.Comments += tot.Total.Comments
			val.Shares += tot.Total.Shares
			val.Spent += tot.Total.Spent
			val.Influencers += tot.Total.Influencers
			val.Perks += tot.Total.Perks
		}
	}

	return tg
}

func GetInfluencerBreakdown(inf influencer.Influencer, cfg *config.Config, offset int, rep map[string]float64, currentRep float64, cid, agid string) map[string]*ReportStats {
	// Retrieves influencer totals for the range and influencer stats by day
	tg := make(map[string]*ReportStats)

	dateRange := common.GetDateRangeFromOffset(offset)

	// Insert totals for the range in the key "totals"
	st := &ReportStats{}
	if cid == "" {
		// Do not add the rep values if we are doing
		// Campaign Influencer stats (i.e. when cid override
		// is not passed in)
		st.Rep = currentRep
	}
	tg["total"] = st

	// Insert day stats for the range
	for _, d := range dateRange {
		r, err := GetInfluencerStats(inf, cfg, d, d, cid, agid)
		if err == nil && r != nil && r.Spent != 0 {
			key := common.GetDateFromTime(d)

			if cid == "" {
				// Do not add the rep values if we are doing
				// Campaign Influencer stats
				dayRep, ok := rep[key]
				if ok {
					r.Rep = dayRep
				}
			}

			if offset != -1 {
				// Do not give day breakdown if it's all time!
				tg[key] = r
			}
			val, _ := tg["total"]
			val.Clicks += r.Clicks
			val.Likes += r.Likes
			val.Comments += r.Comments
			val.Shares += r.Shares
			val.Views += r.Views
			val.Spent += r.Spent
			val.AgencySpent += r.AgencySpent
			val.Engagements += r.Engagements
		}
	}

	return tg
}
