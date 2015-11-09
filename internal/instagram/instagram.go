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

	LastLocation misc.GeoRecord

	LastUpdated int64   // Epoch timestamp in seconds
	PostsSince  []*Post // Posts since last update.. will later check these for deal satisfaction

	Score float32
}

type Post struct {
	Id        string
	Content   string
	Mentions  []string
	Timestamp int32

	// Stats
	Likes    int32
	Comments int32
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
	if in.UserId != "" {
		if fl, err := getFollowers(in.UserId, cfg); err == nil {
			if in.Followers > 0 {
				// Make sure this isn't first run
				in.FollowerDelta = (in.Followers - fl)
			}
			in.Followers = fl
		} else {
			return err
		}

		if likes, cm, err := getPostInfo(in.UserId, cfg); err == nil {
			in.AvgLikes = likes
			in.AvgComments = cm
		} else {
			return err
		}

		in.Score = in.GetScore()
		in.LastUpdated = time.Now().Unix()
	}
	return nil
}

func (in *Instagram) GetScore() float32 {
	return (in.Followers * 3) + (in.FollowerDelta * 2) + (in.AvgComments * 2) + (in.AvgLikes)
}
