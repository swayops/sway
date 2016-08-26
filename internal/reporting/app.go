package reporting

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
)

func GetCampaignBreakdown(cid string, db *bolt.DB, cfg *config.Config, offset int) map[string]*Totals {
	// Retrieves totals for the range and campaign stats by day
	tg := make(map[string]*Totals)

	dateRange := getDateRangeFromOffset(offset)

	// Insert totals for the range in the key "total"
	tg["total"] = &Totals{}

	// Insert day stats for the range
	for _, d := range dateRange {
		tot, err := GetCampaignStats(cid, db, cfg, d, d, true)
		if err == nil && tot != nil && tot.Total != nil {
			tg[getDateFromTime(d)] = tot.Total
			val, _ := tg["total"]
			val.Clicks += tot.Total.Clicks
			val.Engagements += tot.Total.Engagements
			val.Likes += tot.Total.Likes
			val.Views += tot.Total.Views
			val.Comments += tot.Total.Comments
			val.Shares += tot.Total.Shares
			val.Spent += tot.Total.Spent
			val.Influencers = tot.Total.Influencers
		}
	}

	return tg
}

func GetInfluencerBreakdown(infId string, au *auth.Auth, cfg *config.Config, offset int, rep map[string]float64, currentRep float64, cid, agid string) map[string]*ReportStats {
	// Retrieves influencer totals for the range and influencer stats by day
	tg := make(map[string]*ReportStats)

	dateRange := getDateRangeFromOffset(offset)

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
		r, err := GetInfluencerStats(infId, au, cfg, d, d, cid, agid)
		if err == nil && r != nil && r.Spent != 0 {
			key := getDateFromTime(d)

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
