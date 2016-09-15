package reporting

import (
	"strconv"
	"time"

	"github.com/swayops/sway/internal/common"
)

const (
	engagementViewRatio = 0.04
)

func getEngagements(st *common.Stats) int32 {
	return st.Likes + st.Dislikes + st.Comments + st.Shares
}

func getViews(st *common.Stats, eng int32) int32 {
	var views int32
	if st.Views == 0 {
		// There are no concrete views so lets gueestimate!
		// Assume engagement rate is 4% of views!
		views = int32(float64(eng) / engagementViewRatio)
	} else {
		views += st.Views
	}
	return views
}

func GetViews(likes, comments, shares int32) int32 {
	return int32(float64(likes+comments+shares) / engagementViewRatio)
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
			}

			val.Clicks += stats.Clicks
			val.Engagements += stats.Engagements
			val.Likes += stats.Likes
			val.Views += stats.Views
			val.Comments += stats.Comments
			val.Shares += stats.Shares
			val.Spent += stats.Spent
			val.Influencers += stats.Influencers

			tot[date] = val
		}
	}
	return tot
}
