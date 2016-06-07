package facebook

import (
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Facebook struct {
	Id string `json:"id"`

	AvgLikes    float64 `json:"avgLikes,omitempty"`
	AvgComments float64 `json:"avgComments,omitempty"`
	AvgShares   float64 `json:"avgShares,omitempty"`

	Followers     float64 `json:"followers,omitempty"`     // float64 for GetScore equation
	FollowerDelta float64 `json:"followerDelta,omitempty"` // Follower delta since last UpdateData run

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	Score float64 `json:"score,omitempty"`
}

func New(id string, cfg *config.Config) (*Facebook, error) {
	fb := &Facebook{
		Id: id,
	}
	err := fb.UpdateData(cfg)
	return fb, err
}

func (fb *Facebook) UpdateData(cfg *config.Config) error {
	// If we already updated in the last 12 hours, skip
	if misc.WithinLast(fb.LastUpdated, cfg.InfluencerTTL) {
		return nil
	}

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

func (fb *Facebook) GetScore() float64 {
	return (fb.Followers * 3) + (fb.AvgShares * 2) + (fb.AvgComments * 2) + (fb.AvgLikes)
}
