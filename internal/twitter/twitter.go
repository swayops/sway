package twitter

import (
	"github.com/mrjones/oauth"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

const timelineUrl = `https://api.twitter.com/1.1/statuses/user_timeline.json?exclude_replies=true&screen_name=%s`

var (
	serviceProvider = oauth.ServiceProvider{
		RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
		AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
		AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
	}
)

type Twitter struct {
	Id string

	AvgRetweets   float32
	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []misc.GeoRecord // All locations since last update
	LastTweetId  string           // the id of the last tweet
	LatestPosts  []*Post          // Posts since last update.. will later check these for deal satisfaction

	Score float32

	o *oauth.Consumer
}

type Post struct {
	Id        string
	Content   string
	Mentions  []string
	Hashtags  []string
	Published int64 // epoch ts

	PostURL string // Link to the twitter post

	// Stats
	Retweets int32
}

func New(id string, cfg *config.Config) (*Twitter, error) {
	tw := &Twitter{
		Id: id,
		o:  oauth.NewConsumer(cfg.Twitter.Key, cfg.Twitter.Secret, serviceProvider),
	}
	err := tw.UpdateData(cfg.Twitter.Endpoint)
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
		//	tw.LatestPosts = getPosts(tw.LastUpdated) // All posts newer than last updated
		//	tw.LastUpdated = time.Now().Unix()
	}
	return nil
}

func (tw *Twitter) GetTweets() Tweets {
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

func GetStatsByPost(id string) *Post {
	// Each package has this function.. so we can update stats for deal posts
	// Should take in a post Id and return all post stats
	return nil
}
