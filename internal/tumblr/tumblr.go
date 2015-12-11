package tumblr

import (
	"net/http"
	"path"

	"github.com/mrjones/oauth"
	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Tumblr struct {
	Id string

	AvgReblogs    float32
	AvgLikes      float32
	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []misc.GeoRecord // All locations since last update
	LastTweetId  string           // the id of the last tweet
	LatestTweets Posts            // Posts since last update.. will later check these for deal satisfaction
	LastUpdated  int32            // If you see this on year 2038 and wonder why it broke, find Shahzil.
	Score        float32

	client *http.Client
}

func New(id string, cfg *config.Config) (tr *Tumblr, err error) {
	if len(id) == 0 {
		return nil, misc.ErrMissingId
	}

	tr = &Tumblr{Id: id}
	if tr.client, err = getClient(cfg); err != nil {
		return
	}
	//err = tw.UpdateData(cfg.Twitter.Endpoint)
	return
}

func (tr *Tumblr) UpdateData(ep string) error {
	u := path.Join(ep, "blog", tr.Id, "posts")
}

func getClient(cfg *config.Config) (*http.Client, error) {
	c := cfg.Tumblr
	if len(c.Key) == 0 || len(c.Secret) == 0 || len(c.AccessToken) == 0 || len(c.AccessSecret) == 0 || len(c.Endpoint) == 0 {
		return nil, config.ErrInvalidConfig
	}
	oc := oauth.NewConsumer(c.Key, c.Secret, serviceProvider)
	return oc.MakeHttpClient(&oauth.AccessToken{
		Token:  c.AccessToken,
		Secret: c.AccessSecret,
	})
}
