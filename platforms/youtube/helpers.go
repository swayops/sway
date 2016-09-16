package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Meta struct {
	Code int `json:"code"`
}

const (
	searchesUrl  = "%ssearch?part=id&maxResults=1&q=%s&type=channel&key=%s"
	dataUrl      = "%schannels?part=statistics&id=%s&key=%s"
	playlistUrl  = "%schannels?part=contentDetails&forUsername=%s&key=%s"
	videosUrl    = "%splaylistItems?part=snippet&playlistId=%s&key=%s&maxResults=%s"
	postUrl      = "%svideos?id=%s&part=statistics&key=%s"
	postTemplate = "https://www.youtube.com/watch?v=%s"
)

var (
	ErrUnknown = errors.New(`Data not found!`)
	ErrEmpty   = errors.New("Empty post items!")
	ErrStats   = errors.New("Unable to retrieve video stats")
	ErrContent = errors.New("Empty content items")
)

type Data struct {
	Items []*Item `json:"items"`
	Error *Error  `json:"error"`
}

type Error struct {
	Code string `json:"code"`
}

type Item struct {
	Id      Id       `json:"id"`
	Stats   *Stats   `json:"statistics"`
	Content *Content `json:"contentDetails"`
	Snippet *Snippet `json:"snippet"`
}

type Id string

func (id Id) String() string {
	return string(id)
}

func (id *Id) UnmarshalJSON(p []byte) (err error) {
	if len(p) == 0 {
		return // or an error
	}
	if ln := len(p) - 1; p[0] == '"' && p[ln] == '"' {
		*id = Id(p[1:ln])
		return nil
	}
	var tmp struct {
		ChannelId string `json:"channelId"`
	}
	if err = json.Unmarshal(p, &tmp); err == nil {
		*id = Id(tmp.ChannelId)
	}
	return
}

type Stats struct {
	Views      string `json:"viewCount"`
	Likes      string `json:"likeCount"`
	Dislikes   string `json:"dislikeCount"`
	Comments   string `json:"commentCount"`
	Subs       string `json:"subscriberCount"`
	VideoCount string `json:"videoCount"`
}

type Content struct {
	Playlists *Playlist `json:"relatedPlaylists"`
}

type Playlist struct {
	UploadKey string `json:"uploads"`
}

type Snippet struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Published   time.Time `json:"publishedAt"`
	Resource    *Resource `json:"resourceId"`
}

type Resource struct {
	VideoId string `json:"videoId"`
}

func getUserIdFromName(name string, cfg *config.Config) (string, error) {
	endpoint := fmt.Sprintf(searchesUrl, cfg.YouTube.Endpoint, name, cfg.YouTube.ClientId)

	var search Data
	err := misc.Request("GET", endpoint, "", &search)
	if err != nil || search.Error != nil {
		return "", err
	}

	if len(search.Items) > 0 {
		for _, data := range search.Items {
			if data.Id != "" {
				return data.Id.String(), nil
			}
		}
	}

	return "", ErrUnknown
}

func getUserStats(id string, cfg *config.Config) (float64, float64, float64, error) {
	endpoint := fmt.Sprintf(dataUrl, cfg.YouTube.Endpoint, id, cfg.YouTube.ClientId)

	var data Data
	err := misc.Request("GET", endpoint, "", &data)
	if err != nil || data.Error != nil {
		return 0, 0, 0, err
	}

	if len(data.Items) == 0 {
		return 0, 0, 0, err
	}

	stats := data.Items[0].Stats
	if stats == nil {
		return 0, 0, 0, err
	}

	videos, err := getCount(stats.VideoCount)
	if err != nil {
		return 0, 0, 0, err
	}

	views, err := getCount64(stats.Views)
	if err != nil {
		return 0, 0, 0, err
	}

	comments, err := getCount(stats.Comments)
	if err != nil {
		return 0, 0, 0, err
	}

	subs, err := getCount(stats.Subs)
	if err != nil {
		return 0, 0, 0, err
	}

	return views / float64(videos), comments / videos, subs, nil
}

func getPosts(name string, count int, cfg *config.Config) (posts []*Post, avgLikes, avgDislikes float64, err error) {
	endpoint := fmt.Sprintf(playlistUrl, cfg.YouTube.Endpoint, name, cfg.YouTube.ClientId)

	var list Data
	err = misc.Request("GET", endpoint, "", &list)
	if err != nil {
		log.Println("Unable to hit", endpoint)
		return
	}

	if list.Error != nil {
		err = fmt.Errorf("%s: error code: %v", endpoint, list.Error.Code)
		return
	}

	if len(list.Items) == 0 {
		err = ErrEmpty
		return
	}

	val := list.Items[0].Content
	if val == nil || val.Playlists == nil {
		err = ErrContent
		return
	}

	endpoint = fmt.Sprintf(videosUrl, cfg.YouTube.Endpoint, val.Playlists.UploadKey, cfg.YouTube.ClientId, strconv.Itoa(count))
	var vid Data
	err = misc.Request("GET", endpoint, "", &vid)
	if err != nil {
		log.Println("Unable to hit", endpoint)
		return
	}

	if vid.Error != nil {
		err = fmt.Errorf("%s: error code: %v", endpoint, vid.Error.Code)
		return
	}

	if len(vid.Items) == 0 {
		err = ErrEmpty
		return
	}

	for _, v := range vid.Items {
		if v.Snippet != nil && v.Snippet.Resource != nil {
			pub := int32(v.Snippet.Published.Unix())

			p := &Post{
				Id:          v.Snippet.Resource.VideoId,
				Title:       v.Snippet.Title,
				Description: strings.Replace(v.Snippet.Description, "\n", " ", -1),
				Published:   pub,
				LastUpdated: int32(time.Now().Unix()),
				PostURL:     fmt.Sprintf(postTemplate, v.Snippet.Resource.VideoId),
			}

			p.Views, p.Likes, p.Dislikes, p.Comments, err = getVideoStats(v.Snippet.Resource.VideoId, cfg)
			if err != nil {
				continue
			}

			p.LikesDelta = p.Likes
			p.DislikesDelta = p.Dislikes
			p.ViewsDelta = p.Views
			p.CommentsDelta = p.Comments

			avgLikes += p.Likes
			avgDislikes += p.Dislikes

			posts = append(posts, p)
		}
	}

	length := float64(len(posts))
	avgLikes = avgLikes / length
	avgDislikes = avgDislikes / length

	return
}

func getVideoStats(videoId string, cfg *config.Config) (views float64, likes, dislikes, comments float64, err error) {
	endpoint := fmt.Sprintf(postUrl, cfg.YouTube.Endpoint, videoId, cfg.YouTube.ClientId)

	var vData Data
	err = misc.Request("GET", endpoint, "", &vData)
	if err != nil || vData.Error != nil || len(vData.Items) == 0 {
		log.Println("Error extracting video data", endpoint, err)
		err = ErrStats
		return
	}

	i := vData.Items[0]
	if i.Stats == nil {
		log.Println("Error extracting stats data", endpoint, err)
		err = ErrStats
		return
	}

	views, err = getCount64(i.Stats.Views)
	if err != nil {
		log.Println("Error extracting views data", endpoint)
		err = ErrStats
		return
	}

	likes, err = getCount(i.Stats.Likes)
	if err != nil {
		log.Println("Error extracting likes data", endpoint)
		err = ErrStats
		return
	}

	dislikes, err = getCount(i.Stats.Dislikes)
	if err != nil {
		log.Println("Error extracting dislikes data", endpoint)
		err = ErrStats
		return
	}

	comments, err = getCount(i.Stats.Comments)
	if err != nil {
		log.Println("Error extracting comments data", endpoint)
		err = ErrStats
		return
	}

	return
}

func getCount64(val string) (float64, error) {
	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}

	return v, nil
}

func getCount(val string) (float64, error) {
	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}

	return float64(v), nil
}
