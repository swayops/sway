package instagram

import (
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/misc"
)

// AUTH:
// https://api.instagram.com/oauth/authorize/?client_id={{CLIENT_ID}}&redirect_uri=http://lol:8080&response_type=token&scope=basic+public_content

type Instagram struct {
	UserName      string  `json:"userName"`
	UserId        string  `json:"userId"`
	AvgLikes      float64 `json:"avgLikes,omitempty"`    // Per post
	AvgComments   float64 `json:"avgComments,omitempty"` // Per post
	Followers     float64 `json:"followers,omitempty"`
	FollowerDelta float64 `json:"fDelta,omitempty"` // Follower delta since last UpdateData run

	LastLocation *geo.GeoRecord `json:"geo,omitempty"` // All locations since last update

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	LinkInBio string `json:"link,omitempty"`

	Score float64
}

func New(name string, cfg *config.Config) (*Instagram, error) {
	userId, err := getUserIdFromName(name, cfg)
	if err != nil {
		return nil, err
	}

	in := &Instagram{
		UserName: name,
		UserId:   userId,
	}

	err = in.UpdateData(cfg, cfg.Sandbox)
	return in, err
}

func (in *Instagram) UpdateData(cfg *config.Config, savePosts bool) error {
	// Used by an eventual ticker to update stats

	// If we already updated in the last 21-26 hours, skip
	if misc.WithinLast(in.LastUpdated, misc.Random(21, 26)) {
		return nil
	}

	if fl, link, err := getUserInfo(in.UserId, cfg); err == nil {
		if in.Followers > 0 {
			// Make sure this isn't first run
			in.FollowerDelta = (fl - in.Followers)
		}
		in.Followers = fl
		in.LinkInBio = link
	} else {
		return err
	}

	if likes, cm, posts, geo, err := getPostInfo(in.UserId, cfg); err == nil {
		in.AvgLikes = likes
		in.AvgComments = cm

		// Latest posts are only used when there is an active deal!
		if savePosts {
			in.LatestPosts = posts
		} else {
			in.LatestPosts = nil
		}

		if geo != nil {
			in.LastLocation = geo
		}
	} else {
		return err
	}

	in.Score = in.GetScore()
	in.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (in *Instagram) GetScore() float64 {
	return (in.Followers * 3) + (in.AvgComments * 2) + (in.AvgLikes)
}
