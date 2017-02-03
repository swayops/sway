package youtube

import (
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
	dataUrl      = "%schannels?part=statistics,snippet&id=%s&key=%s"
	videosUrl    = "%ssearch?channelId=%s&key=%s&part=snippet,id&order=date&maxResults=20"
	postUrl      = "%svideos?id=%s&part=statistics,snippet&key=%s"
	postTemplate = "https://www.youtube.com/watch?v=%s"
	userUrl      = "%schannels?key=%s&forUsername=%s&part=id"
)

var (
	ErrUnknown = errors.New(`Error adding YouTube channel. Please see this article to find your proper youtube channel ID: http://swayops.com/blog/2016/12/how-to-find-your-true-youtube-id `)
)

type UserData struct {
	Items []*UserItem `json:"items"`
	Error *Error      `json:"error"`
}

type UserItem struct {
	Id      string   `json:"id"`
	Stats   *Stats   `json:"statistics"`
	Content *Content `json:"contentDetails"`
	Snippet *Snippet `json:"snippet"`
}

////

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

type Id struct {
	VideoId   string `json:"videoId"`
	ChannelId string `json:"channelId"`
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
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Published   time.Time  `json:"publishedAt"`
	Thumbnails  *Thumbnail `json:"thumbnails"`
}

type Thumbnail struct {
	Default struct {
		URL string `json:"url"`
	} `json:"default"`
	Medium struct {
		URL string `json:"url"`
	} `json:"medium"`
	High struct {
		URL string `json:"url"`
	} `json:"high"`
	MaxRes struct {
		URL string `json:"url"`
	} `json:"maxres"`
}

func getIdFromUsername(username string, cfg *config.Config) string {
	endpoint := fmt.Sprintf(userUrl, cfg.YouTube.Endpoint, cfg.YouTube.ClientId, username)

	var data UserData
	err := misc.Request("GET", endpoint, "", &data)
	if err != nil || data.Error != nil {
		return ""
	}

	if len(data.Items) > 0 {
		return data.Items[0].Id
	}

	return ""
}

func getUserStats(id string, cfg *config.Config) (float64, float64, float64, string, error) {
	endpoint := fmt.Sprintf(dataUrl, cfg.YouTube.Endpoint, id, cfg.YouTube.ClientId)

	var data UserData
	err := misc.Request("GET", endpoint, "", &data)
	if err != nil || data.Error != nil {
		return 0, 0, 0, "", err
	}

	if len(data.Items) == 0 {
		return 0, 0, 0, "", err
	}

	stats := data.Items[0].Stats
	if stats == nil {
		return 0, 0, 0, "", err
	}

	videos, err := getCount(stats.VideoCount)
	if err != nil {
		return 0, 0, 0, "", err
	}

	views, err := getCount64(stats.Views)
	if err != nil {
		return 0, 0, 0, "", err
	}

	comments, err := getCount(stats.Comments)
	if err != nil {
		return 0, 0, 0, "", err
	}

	subs, err := getCount(stats.Subs)
	if err != nil {
		return 0, 0, 0, "", err
	}

	var url string
	if data.Items[0].Snippet != nil && data.Items[0].Snippet.Thumbnails != nil {
		url = data.Items[0].Snippet.Thumbnails.Medium.URL
	}

	return views / float64(videos), comments / videos, subs, url, nil
}

func getPosts(name string, count int, cfg *config.Config) (posts []*Post, avgLikes, avgDislikes float64, images []string, err error) {
	endpoint := fmt.Sprintf(videosUrl, cfg.YouTube.Endpoint, name, cfg.YouTube.ClientId)

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
		err = ErrUnknown
		return
	}

	var totalLikes, totalDislikes, consideredPosts float64
	for _, v := range vid.Items {
		if v.Snippet != nil && v.Id.VideoId != "" {
			pub := int32(v.Snippet.Published.Unix())

			p := &Post{
				Id:          v.Id.VideoId,
				Title:       v.Snippet.Title,
				Published:   pub,
				LastUpdated: int32(time.Now().Unix()),
				PostURL:     fmt.Sprintf(postTemplate, v.Id.VideoId),
			}

			p.Views, p.Likes, p.Dislikes, p.Comments, p.Description, err = getVideoStats(v.Id.VideoId, cfg)
			if err != nil {
				return
			}

			if v.Snippet.Thumbnails != nil {
				if v.Snippet.Thumbnails.MaxRes.URL != "" {
					images = append(images, v.Snippet.Thumbnails.MaxRes.URL)
				} else if v.Snippet.Thumbnails.High.URL != "" {
					images = append(images, v.Snippet.Thumbnails.High.URL)
				}
			}

			if !misc.WithinLast(p.Published, 24) {
				totalLikes += p.Likes
				totalDislikes += p.Dislikes
				consideredPosts += 1
			}

			posts = append(posts, p)
		}
	}

	if consideredPosts > 0 {
		avgLikes = totalLikes / consideredPosts
		avgDislikes = totalDislikes / consideredPosts
	}

	return
}

func getVideoStats(videoId string, cfg *config.Config) (views float64, likes, dislikes, comments float64, desc string, err error) {
	endpoint := fmt.Sprintf(postUrl, cfg.YouTube.Endpoint, videoId, cfg.YouTube.ClientId)

	var vData UserData
	err = misc.Request("GET", endpoint, "", &vData)
	if err != nil || vData.Error != nil || len(vData.Items) == 0 {
		log.Println("Error extracting video data", endpoint, err)
		err = ErrUnknown
		return
	}

	i := vData.Items[0]
	if i.Stats == nil {
		log.Println("Error extracting stats data", endpoint, err)
		err = ErrUnknown
		return
	}

	views, err = getCount64(i.Stats.Views)
	if err != nil {
		log.Println("Error extracting views data", endpoint)
		err = ErrUnknown
		return
	}

	likes, err = getCount(i.Stats.Likes)
	if err != nil {
		log.Println("Error extracting likes data", endpoint, err)
		err = ErrUnknown
		return
	}

	dislikes, err = getCount(i.Stats.Dislikes)
	if err != nil {
		log.Println("Error extracting dislikes data", endpoint, err)
		err = ErrUnknown
		return
	}

	comments, err = getCount(i.Stats.Comments)
	if err != nil {
		log.Println("Error extracting comments data", endpoint, err)
		err = ErrUnknown
		return
	}

	if i.Snippet != nil && i.Snippet.Description != "" {
		desc = strings.Replace(i.Snippet.Description, "\n", " ", -1)
	}

	return
}

func getCount64(val string) (float64, error) {
	if val == "" {
		return 0, nil
	}

	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}

	return v, nil
}

func getCount(val string) (float64, error) {
	if val == "" {
		return 0, nil
	}

	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}

	return float64(v), nil
}
