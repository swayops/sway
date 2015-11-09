package twitter

import (
	"time"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Twitter struct {
	Id string

	RetweetsPerPost float32
	Followers       float32 // float32 for GetScore equation
	FollowerDelta   float32 // Follower delta since last UpdateData run

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
	Retweets int32
}

func New(id string, cfg *config.Config) (*Twitter, error) {
	tw := &Twitter{
		Id: id,
	}
	err := tw.UpdateData(cfg.TwitterEndpoint)
	return tw, err
}

func (tw *Twitter) UpdateData(endpoint string) error {
	// Used by an eventual ticker to update stats
	if tw.Id != "" {
		if rt, err := getRetweets(tw.Id, endpoint); err == nil {
			tw.RetweetsPerPost = rt
		} else {
			return err
		}

		if fl, err := getFollowers(tw.Id, endpoint); err == nil {
			tw.Followers = fl
		} else {
			return err
		}
		tw.PostsSince = getPosts(tw.LastUpdated)
		tw.LastUpdated = time.Now().Unix()
	}
	return nil
}

func getRetweets(id, endpoint string) (float32, error) {
	return 0, nil
}

func getFollowers(id, endpoint string) (float32, error) {
	return 0, nil
}

func getPosts(last int64) []*Post {
	return nil
}
