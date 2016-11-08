package facebook

import (
	"errors"
	"time"

	"github.com/swayops/sway/config"
)

var (
	ErrEligible  = errors.New("Facebook account is not eligible! Must be a Facebook page (not a personal profile)!")
	ErrFollowers = errors.New("Unfortunately, your Facebook page does not qualify minimum follower count")
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
}

func New(id string, cfg *config.Config) (*Facebook, error) {
	fb := &Facebook{
		Id: id,
	}
	err := fb.UpdateData(cfg, cfg.Sandbox)

	if err != nil {
		return nil, err
	}

	if fb.Followers < 10 {
		return nil, ErrFollowers

	}
	return fb, nil
}

func (fb *Facebook) UpdateData(cfg *config.Config, savePosts bool) error {
	// If we already updated in the last 10-15 hours, skip
	// if misc.WithinLast(fb.LastUpdated, misc.Random(10, 15)) {
	// 	return nil
	// }

	if fb.Id != "" {
		if likes, comments, shares, posts, err := getBasicInfo(fb.Id, cfg); err == nil {
			fb.AvgLikes = likes
			fb.AvgComments = comments
			fb.AvgShares = shares
			// Latest posts are only used when there is an active deal!
			if savePosts {
				fb.LatestPosts = posts
			} else {
				fb.LatestPosts = nil
			}
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
