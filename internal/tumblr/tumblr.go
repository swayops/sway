package tumblr

import (
	"fmt"
	"net/http"

	"github.com/mrjones/oauth"
	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

const (
	allPostsUrl       = `%s/blog/%s/posts?notes_info=true&limit=100`
	allPostsUrlOffset = `%s/blog/%s/posts?notes_info=true&limit=100&offset=%d`
	singlePostUrl     = `%s/blog/%s/posts?notes_info=true&id=%s`
)

type Tumblr struct {
	Id string

	AvgReblogs    float32
	AvgLikes      float32
	Followers     float32 // float32 for GetScore equation
	FollowerDelta float32 // Follower delta since last UpdateData run

	LastLocation []misc.GeoRecord // All locations since last update
	LastPostId   string           // the id of the last tweet
	LatestPosts  Posts            // Posts since last update.. will later check these for deal satisfaction
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
	err = tr.UpdateData(cfg.Twitter.Endpoint, 0)
	return
}

func (tr *Tumblr) UpdateData(ep string, offset int) error {
	posts, err := tr.getPosts(ep, "", offset)
	if err != nil {
		return err
	}
	tr.LatestPosts = posts
	return nil
}

func (tr *Tumblr) getPosts(endpoint, pid string, offset int) (posts Posts, err error) {
	if offset > 0 {
		endpoint = fmt.Sprintf(allPostsUrlOffset, endpoint, tr.Id, offset)
	} else if len(pid) > 0 {
		endpoint = fmt.Sprintf(singlePostUrl, endpoint, tr.Id, pid)
	} else {
		endpoint = fmt.Sprintf(allPostsUrl, endpoint, tr.Id)
	}

	err = misc.HttpGetJson(tr.client, endpoint, &posts)
	return
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
