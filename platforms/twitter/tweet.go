package twitter

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/swayops/sway/config"

	"github.com/swayops/sway/misc"

	"time"
)

type Tweets []*Tweet

func (tws Tweets) Retweets() (n float32) {
	for _, t := range tws {
		n += float32(t.Retweets)
	}
	return
}

func (tws Tweets) AvgRetweets() float32 {
	return tws.Retweets() / float32(len(tws))
}

// likes == favorites
func (tws Tweets) Likes() (n float32) {
	for _, t := range tws {
		n += float32(t.Favorites)
	}
	return
}

func (tws Tweets) AvgLikes() float32 {
	return tws.Likes() / float32(len(tws))
}

func (tws Tweets) Followers() (f float32) {
	if len(tws) > 0 && tws[0].User != nil {
		f = float32(tws[0].User.Followers)
	}
	return
}

func (tws Tweets) LastId() string {
	if len(tws) > 0 && tws[0].User != nil {
		return tws[0].Id
	}
	return ""
}

func (tws Tweets) LatestLocation() *misc.GeoRecord {
	var latest *misc.GeoRecord
	for _, t := range tws {
		if l := t.Location(); l != nil {
			if latest == nil || l.Timestamp > latest.Timestamp {
				latest = l
			}
		}
	}
	return latest
}

type Tweet struct {
	Id        string      `json:"id_str"`
	Retweets  uint32      `json:"retweet_count"`
	Favorites uint32      `json:"favorite_count"`
	CreatedAt TwitterTime `json:"created_at"`

	User *User `json:"user,omitempty"`

	Coords *struct {
		Coords [2]float64 `json:"coordinates"`
	} `json:"coordinates,omitempty"`

	Entities *struct {
		Hashtags []struct {
			Tag string `json:"text"`
		} `json:"hashtags,omitempty"`
	} `json:"entities,omitempty"`

	RetweetedStatus *Tweet `json:"retweeted_status,omitempty"`
}

func (t *Tweet) Location() *misc.GeoRecord {
	if t.Coords == nil {
		return nil
	}
	c := t.Coords.Coords
	return misc.GetGeoFromCoords(c[1], c[0], t.CreatedAt.Unix())
}

func (t *Tweet) Hashtags() (out []string) {
	if t.Entities == nil {
		return
	}
	out = make([]string, 0, len(t.Entities.Hashtags))
	for _, ht := range t.Entities.Hashtags {
		out = append(out, ht.Tag)
	}
	return
}

func (t *Tweet) UpdateData(cfg *config.Config) (err error) {
	var (
		resp   *http.Response
		client *http.Client
	)
	if client, err = getClient(cfg); err != nil {
		return
	}
	if resp, err = client.Get(fmt.Sprintf(tweetUrl, cfg.Twitter.Endpoint, t.Id)); err != nil {
		return
	}
	defer resp.Body.Close()

	r := resp.Body
	if resp.Header.Get("Content-Encoding") != "" {
		var gr *gzip.Reader
		if gr, err = gzip.NewReader(resp.Body); err != nil {
			return
		}
		defer gr.Close()
		r = gr
	}

	var tmp Tweet
	if err = json.NewDecoder(r).Decode(&tmp); err == nil {
		*t = tmp
	}
	return
}

const TwitterTimeLayout = `"Mon Jan 02 15:04:05 -0700 2006"`

type TwitterTime struct {
	time.Time
}

func (t *TwitterTime) UnmarshalJSON(b []byte) (err error) {
	t.Time, err = time.Parse(TwitterTimeLayout, string(b))
	return
}

func (t *TwitterTime) MarshalJSON() ([]byte, error) {
	return []byte(t.Format(TwitterTimeLayout)), nil
}

type User struct {
	Id            string `json:"id_str"`
	Followers     uint32 `json:"followers_count"`
	Friends       uint32 `json:"friends_count"`
	StatusesCount uint32 `json:"statuses_count"`
}
