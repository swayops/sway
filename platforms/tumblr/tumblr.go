package tumblr

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mrjones/oauth"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	allPostsUrl       = `%sblog/%s/posts?notes_info=true&limit=20` // 20 is the max....
	allPostsUrlOffset = `%sblog/%s/posts?notes_info=true&limit=20&offset=%d`
	singlePostUrl     = `%sblog/%s/posts?notes_info=true&id=%s`
)

type Tumblr struct {
	Id string `json:"id"`

	AvgReblogs     float32 `json:"avgRb,omitempty"`
	AvgLikes       float32 `json:"avgLikes,omitempty"`
	AvgInteraction float32 `json:"avgInt,omitempty"`

	LastPostId  string  `json:"lastPost,omitempty"`    // the id of the last tweet
	LatestPosts Posts   `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction
	LastUpdated int32   `json:"lastUpdated,omitempty"` // If you see this on year 2038 and wonder why it broke, find Shahzil.
	Score       float32 `json:"score,omitempty"`

	client *http.Client `json:"client,omitempty"`
}

func New(id string, cfg *config.Config) (tr *Tumblr, err error) {
	if len(id) == 0 {
		return nil, misc.ErrMissingId
	}

	tr = &Tumblr{Id: id}
	if tr.client, err = getClient(cfg); err != nil {
		return
	}
	err = tr.UpdateData(cfg.Tumblr.Endpoint, 0)
	return
}

func (tr *Tumblr) UpdateData(ep string, offset int) error {
	// If we already updated in the last 12 hours, skip
	if misc.WithinLast(tr.LastUpdated, cfg.InfluencerTTL) {
		return nil
	}

	posts, err := tr.getPosts(ep, "", offset)
	if err != nil {
		return err
	}
	tr.LatestPosts = posts
	tr.AvgLikes, tr.AvgReblogs, tr.AvgInteraction = posts.Avgs()
	tr.LastUpdated = int32(time.Now().Unix())
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

	var resp apiResponse
	if err = misc.HttpGetJson(tr.client, endpoint, &resp); err != nil {
		return
	}
	if resp.Meta.Status != 200 {
		return nil, errors.New(resp.Meta.Message)
	}
	return resp.Response.Posts, nil
}

func (tr *Tumblr) GetScore() float32 {
	return (tr.AvgReblogs * 2) + (tr.AvgLikes * 2) + tr.AvgInteraction
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
