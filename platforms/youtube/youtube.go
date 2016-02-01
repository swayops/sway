package youtube

import (
	"time"

	"github.com/swayops/sway/config"
)

type YouTube struct {
	UserName string `json:"name"`
	UserId   string `json:"id"`

	AvgLikes    float32 `json:"avgLikes,omitempty"`
	AvgDislikes float32 `json:"avgDislikes,omitempty"`

	AvgViews        float32 `json:"avgViews,omitempty"`
	AvgComments     float32 `json:"avgComments,omitempty"`
	Subscribers     float32 `json:"avgSub,omitempty"`   // float32 for GetScore equation
	SubscriberDelta float32 `json:"subDelta,omitempty"` // Follower delta since last UpdateData run

	LastUpdated int32   `json:"lastUpdate,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`      // Posts since last update.. will later check these for deal satisfaction

	Score float32 `json:"score,omitempty"`
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
	// Used by an eventual ticker to update stats
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

	p, lk, dlk, err := getPosts(yt.UserName, 10, yt.LastUpdated, cfg)
	if err != nil {
		return err
	}
	yt.LatestPosts = p
	yt.AvgLikes = lk
	yt.AvgDislikes = dlk

	yt.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (yt *YouTube) GetScore() float32 {
	return ((yt.AvgComments * 4) + (yt.AvgLikes * 3) + (yt.Subscribers * 2) + (yt.AvgViews)) / yt.AvgDislikes
}
