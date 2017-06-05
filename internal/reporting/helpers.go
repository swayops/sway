package reporting

import (
	"strconv"
	"time"

	"github.com/swayops/sway/internal/common"
)

func getEngagements(st *common.Stats) int32 {
	return st.Likes + st.Dislikes + st.Comments + st.Shares
}

func GetReportDate(date string) time.Time {
	// YYYY-MM-DD
	if t, err := time.Parse(`2006-01-02`, date); err == nil {
		return t
	}
	if t, err := time.Parse(`02 Jan 06`, date); err == nil {
		return t
	}
	if u, err := strconv.ParseInt(date, 10, 64); err == nil {
		return time.Unix(u, 0)
	}
	return time.Time{}
}

func getPostDate(ts int32) string {
	return time.Unix(int64(ts), 0).String()
}

func Merge(totals []map[string]*Totals) map[string]*Totals {
	// Used for merging stats from multiple campaigns/influencers
	tot := make(map[string]*Totals)
	for _, val := range totals {
		for date, stats := range val {
			val, ok := tot[date]
			if !ok {
				val = &Totals{}
				tot[date] = val
			}
			val.Clicks += stats.Clicks
			val.Uniques += stats.Uniques
			val.Engagements += stats.Engagements
			val.Likes += stats.Likes
			val.Views += stats.Views
			val.Comments += stats.Comments
			val.Shares += stats.Shares
			val.Spent += stats.Spent
			val.Influencers += stats.Influencers
			val.Perks += stats.Perks
		}
	}
	return tot
}
