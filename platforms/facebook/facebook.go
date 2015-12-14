package facebook

import (
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Facebook struct {
	Id string

	AvgLikes    float32
	AvgComments float32
	AvgShares   float32

	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation misc.GeoRecord

	LastUpdated int32   // Epoch timestamp in seconds
	LatestPosts []*Post // Posts since last update.. will later check these for deal satisfaction

	Score float32
}

func New(id string, cfg *config.Config) (*Facebook, error) {
	fb := &Facebook{
		Id: id,
	}
	err := fb.UpdateData(cfg)
	return fb, err
}

func (fb *Facebook) UpdateData(cfg *config.Config) error {
	// Used by an eventual ticker to update stats
	if fb.Id != "" {
		if likes, comments, shares, posts, err := getBasicInfo(fb.Id, cfg); err == nil {
			fb.AvgLikes = likes
			fb.AvgComments = comments
			fb.AvgShares = shares
			fb.LatestPosts = posts
		} else {
			return err
		}

		if fl, err := getFollowers(fb.Id, cfg); err == nil {
			if fb.Followers > 0 {
				fb.FollowerDelta = (fl - fb.Followers)
			}
			fb.Followers = fl
		} else {
			return err
		}
		fb.LastUpdated = int32(time.Now().Unix())
	}
	return nil
}
