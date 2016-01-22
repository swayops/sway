package facebook

import (
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Facebook struct {
	Id string `json:"id"`

	AvgLikes    float32 `json:"avgLikes,omitempty"`
	AvgComments float32 `json:"avgCom,omitempty"`
	AvgShares   float32 `json:"avgSh,omitempty"`

	Followers     float32 `json:"followers,omitempty"` // float32 for GetScore equation
	FollowerDelta float32 `json:"fDelta,omitempty"`    // Follower delta since last UpdateData run

	LastLocation misc.GeoRecord `json:"lastLoc,omitempty"`

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	Score float32 `json:"score,omitempty"`
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
