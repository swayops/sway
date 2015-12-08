package tumblr

import (
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/mrjones/oauth"
	"github.com/swayops/sway/internal/config"
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

type Post struct {
	ID        big.Int   `json:"id"`
	Slug      string    `json:"slug"`
	Type      string    `json:"type"`
	TS        Timestamp `json:"timestamp"`
	NoteCount uint32    `json:"note_count"`
	Tags      []string  `json:"tags"`
	Notes     []Note    `json:"notes"`
}

type Note struct {
	Type string `json:"type"`
	// TS   Timestamp `json:"timestamp"` // is this even needed?
}

// Counts returns the number of reblogs/likes of the most recent 50 notes, API limitation. :(
func (p *Post) Counts() (reblog, likes int) {
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
	return
}

type apiResponse struct {
	Meta struct {
		Status  int    `json:"status"`
		Message string `json:"msg"`
	}
	Blog struct {
		Title    string `json:"title"`
		NumPosts int    `json:"posts"`
	}
	Posts []*Post `json:"posts"`
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
