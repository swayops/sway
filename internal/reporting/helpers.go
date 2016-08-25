package reporting

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/swayops/sway/internal/common"
)

const (
	dateFormat          = "%d-%02d-%02d"
	engagementViewRatio = 0.04
)

func GetDate() string {
	return getDateFromTime(time.Now().UTC())
}

func getDateFromTime(t time.Time) string {
	return fmt.Sprintf(
		dateFormat,
		t.Year(),
		t.Month(),
		t.Day(),
	)
}

func getStatsKey(deal *common.Deal, platformId string) string {
	var platform string
	url := deal.PostUrl
	if deal.Tweet != nil {
		platform = "Twitter"
	} else if deal.Facebook != nil {
		platform = "Facebook"
	} else if deal.Instagram != nil {
		platform = "Instagram"
	} else if deal.YouTube != nil {
		platform = "YouTube"
	} else {
		return ""
	}

	return fmt.Sprintf("%s|||%s|||%s|||%s|||%s", GetDate(), deal.InfluencerId, platformId, platform, url)
}

func getElementsFromKey(s string) (string, string, string, string) {
	raw := strings.Split(s, "|||")
	if len(raw) != 5 {
		return "", "", "", ""
	}

	return raw[1], raw[2], raw[3], raw[4]
}

func getDateRange(from, to time.Time) []string {
	out := []string{}
	diff := to.Sub(from).Hours() / 24

	for i := 0; i <= int(diff); i++ {
		out = append(out, getDateFromTime(from.AddDate(0, 0, i)))
	}
	return out
}

func getPostDate(ts int32) string {
	return time.Unix(int64(ts), 0).String()
}

func getDateRangeFromOffset(off int) []time.Time {
	to := time.Now().UTC()
	if off == -1 {
		off = -365
	} else if off > 0 {
		off = -off
	}
	out := make([]time.Time, -off+1)
	for i := range out {
		out[i] = to.AddDate(0, 0, off+i)
	}
	return out
}

func getEngagements(st *Stats) int32 {
	return st.Likes + st.Dislikes + st.Comments + st.Shares
}

func getViews(st *Stats, eng int32) int32 {
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
