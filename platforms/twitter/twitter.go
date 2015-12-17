package twitter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mrjones/oauth"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	timelineUrl        = `%sstatuses/user_timeline.json?exclude_replies=true&screen_name=%s`
	timelineSinceIdUrl = `%sstatuses/user_timeline.json?exclude_replies=true&screen_name=%s&since_id=%s`
	tweetUrl           = `%sstatuses/show.json?include_entities=false&trim_user=true&id=%s`
)

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
	AvgLikes      float32
	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []misc.GeoRecord // All locations since last update
	LastTweetId  string           // the id of the last tweet
	LatestTweets Tweets           // Posts since last update.. will later check these for deal satisfaction
	LastUpdated  int32            // If you see this on year 2038 and wonder why it broke, find Shahzil.
	Score        float32

	client *http.Client
}

func New(id string, cfg *config.Config) (tw *Twitter, err error) {
	if len(id) == 0 {
		return nil, misc.ErrMissingId
	}

	tw = &Twitter{Id: id}
	if tw.client, err = getClient(cfg); err != nil {
		return
	}
	err = tw.UpdateData(cfg.Twitter.Endpoint)
	return
}

func (tw *Twitter) UpdateData(endpoint string) error {
	tws, err := tw.getTweets(endpoint, tw.LastTweetId)
	if err != nil {
		return err
	}
	tw.AvgRetweets = tws.AvgRetweets()
	tw.AvgLikes = tws.AvgLikes()
	nf := tws.Followers()
	if tw.Followers > 0 {
		tw.FollowerDelta = nf - tw.Followers
	}
	tw.Followers = nf

	tw.LatestTweets = tws
	tw.LastTweetId = tws.LastId()
	tw.Score = tw.GetScore()

	tw.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (tw *Twitter) getTweets(endpoint, lastTweetId string) (tws Tweets, err error) {
	if len(lastTweetId) > 0 {
		endpoint = fmt.Sprintf(timelineSinceIdUrl, endpoint, tw.Id, lastTweetId)
	} else {
		endpoint = fmt.Sprintf(timelineUrl, endpoint, tw.Id)
	}

	err = misc.HttpGetJson(tw.client, endpoint, &tws)
	return
}

func (tw *Twitter) GetScore() float32 {
	return (tw.Followers * 3) + (tw.FollowerDelta * 2) + (tw.AvgRetweets * 2) + (tw.AvgLikes)
}

func getClient(cfg *config.Config) (*http.Client, error) {
	c := cfg.Twitter
	if len(c.Key) == 0 || len(c.Secret) == 0 || len(c.AccessToken) == 0 || len(c.AccessSecret) == 0 || len(c.Endpoint) == 0 {
		return nil, config.ErrInvalidConfig
	}
	oc := oauth.NewConsumer(c.Key, c.Secret, serviceProvider)
	return oc.MakeHttpClient(&oauth.AccessToken{
		Token:  c.AccessToken,
		Secret: c.AccessSecret,
	})
}
