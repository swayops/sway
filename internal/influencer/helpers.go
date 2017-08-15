package influencer

import (
	"fmt"
	"strings"
	"time"

	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

const dateFormat = "%d-%02d"

func getDate() string {
	return getDateFromTime(time.Now().UTC())
}

func getDateFromTime(t time.Time) string {
	return fmt.Sprintf(
		dateFormat,
		t.Year(),
		t.Month(),
	)
}

func degradeRep(val int, rep float64) float64 {
	if val > 0 && val < 5 {
		rep = rep * 0.75
	} else if val >= 5 && val < 20 {
		rep = rep * 0.5
	} else if val >= 20 && val < 50 {
		rep = rep * 0.25
	} else if val >= 50 {
		rep = rep * 0.05
	}
	return rep
}

func clean(user string) string {
	return strings.Replace(user, " ", "", -1)
}

func GetMaxYield(cmp *common.Campaign, yt *youtube.YouTube, fb *facebook.Facebook, tw *twitter.Twitter, insta *instagram.Instagram) float64 {
	if cmp != nil && cmp.IsProductBasedBudget() {
		return 0
	}

	// Expected value on average a post generates
	// NOTE: Priority here is the same as GetAvailableDeals priority for platforms
	if (cmp == nil || cmp.Instagram) && insta != nil {
		yield := insta.AvgLikes * budget.INSTA_LIKE
		yield += insta.AvgComments * budget.INSTA_COMMENT
		return yield
	}

	if (cmp == nil || cmp.YouTube) && yt != nil {
		yield := yt.AvgViews * budget.YT_VIEW
		yield += yt.AvgComments * budget.YT_COMMENT
		yield += yt.AvgLikes * budget.YT_LIKE
		yield += yt.AvgDislikes * budget.YT_DISLIKE
		return yield
	}

	if (cmp == nil || cmp.Twitter) && tw != nil {
		yield := tw.AvgLikes * budget.TW_FAVORITE
		yield += tw.AvgRetweets * budget.TW_RETWEET
		return yield
	}

	if (cmp == nil || cmp.Facebook) && fb != nil {
		yield := fb.AvgLikes * budget.FB_LIKE
		yield += fb.AvgComments * budget.FB_COMMENT
		yield += fb.AvgShares * budget.FB_SHARE
		return yield
	}

	return 0
}
