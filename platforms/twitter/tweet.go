package twitter

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/misc"

	"time"
)

type Tweets []*Tweet

func (tws Tweets) Retweets() (n float64) {
	for _, t := range tws {
		n += float64(t.Retweets)
	}
	return
}

func (tws Tweets) AvgRetweets() float64 {
	return tws.Retweets() / float64(len(tws))
}

// likes == favorites
func (tws Tweets) Likes() (n float64) {
	for _, t := range tws {
		n += float64(t.Favorites)
	}
	return
}

func (tws Tweets) AvgLikes() float64 {
	return tws.Likes() / float64(len(tws))
}

func (tws Tweets) Followers() (f float64) {
	if len(tws) > 0 && tws[0].User != nil {
		f = float64(tws[0].User.Followers)
	}
	return
}

func (tws Tweets) LastId() string {
	if len(tws) > 0 && tws[0].User != nil {
		return tws[0].Id
	}
	return ""
}

func (tws Tweets) ProfilePicture() string {
	if len(tws) > 0 && tws[0].User != nil {
		// Hack to get a larger picture
		return strings.Replace(tws[0].User.ProfilePicture, "_normal", "", -1)
	}
	return ""
}

func (tws Tweets) Name() string {
	if len(tws) > 0 && tws[0].User != nil {
		return tws[0].User.Name
	}
	return ""
}

func (tws Tweets) LatestLocation() *geo.GeoRecord {
	var latest *geo.GeoRecord
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
	Id string `json:"id_str"`

	Retweets      uint32 `json:"retweet_count"`
	RetweetsDelta uint32 `json:"rtDelta"`

	Favorites      uint32 `json:"favorite_count"`
	FavoritesDelta uint32 `json:"fDelta"`

	CreatedAt TwitterTime `json:"created_at"`

	User *User `json:"user,omitempty"`

	Coords *struct {
		Coords [2]float64 `json:"coordinates"`
	} `json:"coordinates,omitempty"`

	Entities *struct {
		Hashtags []struct {
			Tag string `json:"text"`
		} `json:"hashtags,omitempty"`
		Mentions []struct {
			Name string `json:"screen_name"`
		} `json:"user_mentions,omitempty"`
		Urls []struct {
			Url string `json:"expanded_url"`
		} `json:"urls,omitempty"`
	} `json:"entities,omitempty"`

	Text string `json:"text"`
	// RetweetedStatus *Tweet `json:"retweeted_status,omitempty"`

	LastUpdated int32  `json:"lastUpdated,omitempty"`
	PostURL     string `json:"postURL,omitempty"`

	Errors []struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"errors,omitempty"`
}

func (t *Tweet) Location() *geo.GeoRecord {
	if t.Coords == nil {
		return nil
	}
	c := t.Coords.Coords
	return geo.GetGeoFromCoords(c[1], c[0], int32(t.CreatedAt.Unix()))
}

func (t *Tweet) Hashtags() (out []string) {
	if t.Entities == nil {
		return
	}
	out = make([]string, 0, len(t.Entities.Hashtags))
	for _, ht := range t.Entities.Hashtags {
		out = append(out, strings.ToLower(ht.Tag))
	}

	out = misc.SanitizeHashes(out)
	return
}

func (t *Tweet) Mentions() (out []string) {
	if t.Entities == nil {
		return
	}
	out = make([]string, 0, len(t.Entities.Mentions))
	for _, mt := range t.Entities.Mentions {
		out = append(out, strings.ToLower(mt.Name))
	}
	return
}

func (t *Tweet) Urls() (out []string) {
	if t.Entities == nil {
		return
	}
	out = make([]string, 0, len(t.Entities.Urls))
	for _, mt := range t.Entities.Urls {
		out = append(out, strings.ToLower(mt.Url))
	}
	return
}

func (t *Tweet) Clear() {
	t.RetweetsDelta, t.FavoritesDelta = 0, 0
}

func (t *Tweet) UpdateData(cfg *config.Config) (ban, err error) {
	// // If the post is more than 4 days old AND
	// // it has been updated in the last week, SKIP!
	// // i.e. only update old posts once a week
	// if !misc.WithinLast(int32(t.CreatedAt.Unix()), 24*4) && misc.WithinLast(int32(t.CreatedAt.Unix()), 24*7) {
	// 	return nil
	// }

	// // If we have already updated within the last 12 hours, skip!
	// if misc.WithinLast(t.LastUpdated, 12) {
	// 	return nil
	// }

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
	if err = json.NewDecoder(r).Decode(&tmp); err != nil {
		return
	}

	for _, er := range tmp.Errors {
		if er.Code == 144 {
			// Invalid tweet id error code
			ban = errors.New(er.Message)
			return
		}
	}

	t.FavoritesDelta = tmp.Favorites - t.Favorites
	t.Favorites = tmp.Favorites

	t.RetweetsDelta = tmp.Retweets - t.Retweets
	t.Retweets = tmp.Retweets

	t.LastUpdated = int32(time.Now().Unix())
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
	Id             string `json:"id_str"`
	Followers      uint32 `json:"followers_count"`
	ProfilePicture string `json:"profile_image_url_https"`
	Name           string `json:"name"`

	// Friends   uint32 `json:"friends_count"`
	// StatusesCount uint32 `json:"statuses_count"`
}
