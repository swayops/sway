package twitter

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/mrjones/oauth"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

const (
	timelineUrl = `https://api.twitter.com/1.1/statuses/user_timeline.json?exclude_replies=true&screen_name=`
	tweetUrl    = `https://api.twitter.com/1.1/statuses/show.json?include_entities=false&trim_user=true&id=`
)

var (
	ErrMissingId    = errors.New("missing id")
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
		return nil, ErrMissingId
	}
	tCfg := cfg.Twitter
	if len(tCfg.Key) == 0 || len(tCfg.Secret) == 0 || len(tCfg.AccessToken) == 0 || len(tCfg.AccessSecret) == 0 || len(tCfg.Endpoint) == 0 {
		return nil, config.ErrInvalidConfig
	}

	oc := oauth.NewConsumer(tCfg.Key, tCfg.Secret, serviceProvider)
	tw = &Twitter{Id: id}
	if tw.client, err = oc.MakeHttpClient(&oauth.AccessToken{
		Token:  tCfg.AccessToken,
		Secret: tCfg.AccessSecret,
	}); err != nil {
		return
	}
	err = tw.UpdateData(tCfg.Endpoint)
	return
}

func (tw *Twitter) UpdateData(endpoint string) error {
	tws, err := tw.GetTweets(tw.LastTweetId)
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

func (tw *Twitter) GetTweets(lastTweetId string) (tws Tweets, err error) {
	url := timelineUrl + tw.Id
	if len(lastTweetId) > 0 {
		url += "&since_id=" + lastTweetId
	}
	//log.Println(url)
	var resp *http.Response
	if resp, err = tw.client.Get(url); err != nil {
		return
	}
	defer resp.Body.Close()
	var gr *gzip.Reader
	if gr, err = gzip.NewReader(resp.Body); err != nil {
		return
	}
	err = json.NewDecoder(gr).Decode(&tws)
	gr.Close()
	return
}

func (tw *Twitter) GetTweet(id string) (t *Tweet, err error) {
	var resp *http.Response
	if resp, err = tw.client.Get(tweetUrl + id); err != nil {
		return
	}
	defer resp.Body.Close()
	var gr *gzip.Reader
	if gr, err = gzip.NewReader(resp.Body); err != nil {
		return
	}
	err = json.NewDecoder(gr).Decode(&t)
	gr.Close()
	return
}

func (tw *Twitter) GetScore() float32 {
	return (tw.Followers * 3) + (tw.FollowerDelta * 2) + (tw.AvgRetweets * 2) + (tw.AvgLikes)
}
