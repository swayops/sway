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
	timelineUrl        = `%sstatuses/user_timeline.json?exclude_replies=true&screen_name=%s&count=20`
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
	Id string `json:"id"`

	AvgRetweets   float64 `json:"avgRt,omitempty"`
	AvgLikes      float64 `json:"avgLikes,omitempty"`
	Followers     float64 `json:"followers,omitempty"` // float64 for GetScore equation
	FollowerDelta float64 `json:"fDelta,omitempty"`    // Follower delta since last UpdateData run

	LastLocation *misc.GeoRecord `json:"geo,omitempty"`
	LastTweetId  string          `json:"lastTw,omitempty"`      // the id of the last tweet
	LatestTweets Tweets          `json:"latestTw,omitempty"`    // Posts since last update.. will later check these for deal satisfaction
	LastUpdated  int32           `json:"lastUpdated,omitempty"` // If you see this on year 2038 and wonder why it broke, find Shahzil.
	Score        float64         `json:"score,omitempty"`

	client *http.Client `json:"client,omitempty"`
}

func New(id string, cfg *config.Config) (tw *Twitter, err error) {
	if len(id) == 0 {
		return nil, misc.ErrMissingId
	}

	tw = &Twitter{Id: id}
	if tw.client, err = getClient(cfg); err != nil {
		return
	}
	err = tw.UpdateData(cfg)
	return
}

func (tw *Twitter) UpdateData(cfg *config.Config) error {
	// If we already updated in the last 12 hours, skip
	if misc.WithinLast(tw.LastUpdated, 12) {
		return nil
	}

	var err error
	if tw.client == nil {
		if tw.client, err = getClient(cfg); err != nil {
			return err
		}
	}

	tws, err := tw.getTweets(cfg.Twitter.Endpoint)
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
	tw.LastLocation = tws.LatestLocation()

	tw.LastUpdated = int32(time.Now().Unix())
	return nil
}

const (
	postURL = "https://twitter.com/%s/status/%s"
)

func (tw *Twitter) getTweets(endpoint string) (tws Tweets, err error) {
	endpoint = fmt.Sprintf(timelineUrl, endpoint, tw.Id)
	err = misc.HttpGetJson(tw.client, endpoint, &tws)
	now := int32(time.Now().Unix())
	for _, t := range tws {
		t.LastUpdated = now
		if t.User != nil {
			t.PostURL = fmt.Sprintf(postURL, t.User.Id, t.Id)
		}
		t.RetweetsDelta = t.Retweets
		t.FavoritesDelta = t.Favorites

	}
	return
}

func (tw *Twitter) GetScore() float64 {
	return (tw.Followers * 3) + (tw.AvgRetweets * 2) + (tw.AvgLikes * 2)
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
