package youtube

import (
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type YouTube struct {
	UserName string `json:"name"`
	UserId   string `json:"id"`

	AvgLikes    float64 `json:"avgLikes,omitempty"`
	AvgDislikes float64 `json:"avgDislikes,omitempty"`

	AvgViews        float64 `json:"avgViews,omitempty"`
	AvgComments     float64 `json:"avgComments,omitempty"`
	Subscribers     float64 `json:"avgSubs,omitempty"`  // float64 for GetScore equation
	SubscriberDelta float64 `json:"subDelta,omitempty"` // Follower delta since last UpdateData run

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	Score float64 `json:"score,omitempty"`
}

func New(name string, cfg *config.Config) (*YouTube, error) {
	userId, err := getUserIdFromName(name, cfg)
	if err != nil {
		return nil, err
	}

	yt := &YouTube{
		UserName: name,
		UserId:   userId,
	}

	err = yt.UpdateData(cfg)
	return yt, err
}

func (yt *YouTube) UpdateData(cfg *config.Config) error {
	// If we already updated in the last 12 hours, skip
	if misc.WithinLast(yt.LastUpdated, cfg.InfluencerTTL) {
		return nil
	}

	if views, comments, subs, err := getUserStats(yt.UserId, cfg); err == nil {
		if yt.Subscribers > 0 {
			yt.SubscriberDelta = (subs - yt.Subscribers)
		}
		yt.AvgViews = views
		yt.AvgComments = comments
		yt.Subscribers = subs
	} else {
		return err
	}

	p, lk, dlk, err := getPosts(yt.UserName, 10, cfg)
	if err != nil {
		return err
	}
	yt.LatestPosts = p
	yt.AvgLikes = lk
	yt.AvgDislikes = dlk

	yt.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (yt *YouTube) GetScore() float64 {
	return (yt.Subscribers * 2.5) + (yt.AvgComments * 1.5) + float64(yt.AvgLikes) + float64(yt.AvgViews)
}
