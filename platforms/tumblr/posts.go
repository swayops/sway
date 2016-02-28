package tumblr

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/mrjones/oauth"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

var (
	serviceProvider = oauth.ServiceProvider{
		RequestTokenUrl:   "https://www.tumblr.com/oauth/request_token",
		AuthorizeTokenUrl: "https://www.tumblr.com/oauth/authorize",
		AccessTokenUrl:    "https://www.tumblr.com/oauth/access_token",
	}
)

type Timestamp int64

func (ts Timestamp) Time() time.Time {
	return time.Unix(int64(ts), 0)
}

type Posts []*Post

func (posts Posts) Avgs() (reblog, likes, total float32) {
	for _, p := range posts {
		r, l, t := p.Counts()
		reblog += r
		likes += l
		total += t
	}
	ln := float32(len(posts))
	return reblog / ln, likes / ln, total / ln
}

type Post struct {
	ID        big.Int   `json:"id"`
	Type      string    `json:"type"`
	TS        Timestamp `json:"timestamp"`
	NoteCount uint32    `json:"note_count"`
	Tags      []string  `json:"tags"`
	Notes     []Note    `json:"notes"`

	LastUpdated int32 `json:"lastUpdated,omitempty"`
}

type Note struct {
	Type string `json:"type"`
}

// Counts returns the number of reblogs/likes of the most recent 50 notes, API limitation. :(
func (p *Post) Counts() (reblog, likes, total float32) {
	for i := range p.Notes {
		switch p.Notes[i].Type {
		case "like":
			likes++
		case "reblog", "posted":
			reblog++
		default:
			log.Printf("unknown type: %s", p.Notes[i].Type)
		}
	}
	total = float32(p.NoteCount)
	return
}

// UpdateData needs the parent tumblr call because it needs the blog id
func (p *Post) UpdateData(tr *Tumblr, cfg *config.Config) (err error) {
	// If the post is more than 4 days old AND
	// it has been updated in the last week, SKIP!
	// i.e. only update old posts once a week
	if !misc.WithinLast(int32(p.TS.Unix()), 24*4) && misc.WithinLast(int32(p.TS.Unix()), 24*7) {
		return nil
	}

	// If we have already updated within the last 12 hours, skip!
	if misc.WithinLast(p.LastUpdated, 12) {
		return nil
	}

	var resp apiResponse
	if err = misc.HttpGetJson(tr.client, fmt.Sprintf(singlePostUrl, cfg.Tumblr.Endpoint, tr.Id, p.ID.String()), &resp); err != nil {
		return
	}
	if resp.Meta.Status != 200 {
		return errors.New(resp.Meta.Message)
	}
	if len(resp.Response.Posts) == 1 { // should never be more or less than 1
		*p = *resp.Response.Posts[0]
	}
	p.LastUpdated = int32(time.Now().Unix())

	return
}

type apiResponse struct {
	Meta struct {
		Status  int    `json:"status"`
		Message string `json:"msg"`
	}
	Response struct {
		Blog struct {
			Title    string `json:"title"`
			NumPosts int    `json:"posts"`
		}
		Posts Posts `json:"posts"`
	} `json:"response"`
}
