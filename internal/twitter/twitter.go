package twitter

import (
	"time"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Twitter struct {
	Id string

	AvgRetweets   float32
	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []misc.GeoRecord // All locations since last update
	LastUpdated  int64            // Epoch timestamp in seconds
	LatestPosts  []*Post          // Posts since last update.. will later check these for deal satisfaction

	Score float32
}

type Post struct {
	Id        string
	Content   string
	Mentions  []string
	Hashtags  []string
	Published int32 // epoch ts

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
			tw.AvgRetweets = rt
		} else {
			return err
		}

		if fl, err := getFollowers(tw.Id, endpoint); err == nil {
			tw.Followers = fl
		} else {
			return err
		}
		tw.LatestPosts = getPosts(tw.LastUpdated) // All posts newer than last updated
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
