package youtube

import (
	"time"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type YouTube struct {
	UserName string
	UserId   string

	AvgLikes    float32
	AvgDislikes float32

	AvgViews        float32
	AvgComments     float32
	Subscribers     float32 // float32 for GetScore equation
	SubscriberDelta float32 // Follower delta since last UpdateData run

	LastLocation misc.GeoRecord

	LastUpdated int32   // Epoch timestamp in seconds
	LatestPosts []*Post // Posts since last update.. will later check these for deal satisfaction

	Score float32
}

type Post struct {
	Id          string
	Title       string
	Description string
	Published   int32 // Epoch ts

	// Stats
	Views    float32
	Likes    float32
	Dislikes float32
	Comments float32
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
			yt.SubscriberDelta = (yt.Subscribers - subs)
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
	return ((yt.Subscribers * 3) + (yt.AvgViews * 3) + (yt.AvgComments * 2) + (yt.AvgLikes)) / yt.AvgDislikes
}
