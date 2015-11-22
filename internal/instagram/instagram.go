package instagram

import (
	"time"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Instagram struct {
	UserName      string
	UserId        string
	AvgLikes      float32 // Per post
	AvgComments   float32 // Per post
	Followers     float32
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []*misc.GeoRecord // All locations since last update

	LastUpdated int32   // Epoch timestamp in seconds
	LatestPosts []*Post // Posts since last update.. will later check these for deal satisfaction

	Score float32
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

	err = in.UpdateData(cfg)
	return in, err
}

func (in *Instagram) UpdateData(cfg *config.Config) error {
	// Used by an eventual ticker to update stats
	if fl, err := getFollowers(in.UserId, cfg); err == nil {
		if in.Followers > 0 {
			// Make sure this isn't first run
			in.FollowerDelta = (fl - in.Followers)
		}
		in.Followers = fl
	} else {
		return err
	}

	if likes, cm, posts, geos, err := getPostInfo(in.UserId, in.LastUpdated, cfg); err == nil {
		in.AvgLikes = likes
		in.AvgComments = cm
		in.LatestPosts = posts
		in.LastLocation = geos
	} else {
		return err
	}

	in.Score = in.GetScore()
	in.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (in *Instagram) GetScore() float32 {
	return (in.Followers * 3) + (in.FollowerDelta * 2) + (in.AvgComments * 2) + (in.AvgLikes)
}
