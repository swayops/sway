package instagram

import (
	"errors"
	"fmt"
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
	postCount    = 30.0
	searchesUrl  = "%susers/search?q=%s&client_id=%s"
	followersUrl = "%susers/%s/?client_id=%s"
	postUrl      = "%susers/%s/media/recent/?client_id=%s&count=30"
	postIdUrl    = "%smedia/%s?client_id=%s"
)

var (
	ErrCode    = errors.New(`Non-200 Instagram Status Code`)
	ErrUnknown = errors.New(`Data not found!`)
)

type UserSearch struct {
	Meta *Meta         `json:"meta"`
	Data []*SearchData `json:"data"`
}

type SearchData struct {
	Name string `json:"username"`
	Id   string `json:"id"`
}

func getUserIdFromName(name string, cfg *config.Config) (string, error) {
	endpoint := fmt.Sprintf(searchesUrl, cfg.Instagram.Endpoint, name, cfg.Instagram.ClientId)

	var search UserSearch
	err := misc.Request("GET", endpoint, "", &search)
	if err != nil {
		return "", err
	}

	if search.Meta.Code != 200 {
		return "", ErrCode
	}

	if len(search.Data) > 0 {
		for _, data := range search.Data {
			if strings.ToLower(data.Name) == strings.ToLower(name) {
				return strings.ToLower(data.Id), nil
			}
		}
	}

	return "", ErrUnknown
}

type UserPost struct {
	Meta *Meta       `json:"meta"`
	Data []*PostData `json:"data"`
}

type PostData struct {
	Id        string   `json:"id"`
	Tags      []string `json:"tags"`
	Published string   `json:"created_time"`
	URL       string   `json:"link"`

	Comments *Comments `json:"comments"`
	Likes    *Likes    `json:"likes"`
	Location *Location `json:"location"`
	Caption  *Caption  `json:"caption"`
}

type Location struct {
	Latitude   float64 `json:"latitude"`
	Longtitude float64 `json:"longitude"`
	Name       string  `json:"name"`
}

type Comments struct {
	Count float32 `json:"count"`
}

type Likes struct {
	Count float32 `json:"count"`
}

type Caption struct {
	Msg string `json:"text"`
}

func getPostInfo(id string, cfg *config.Config) (float32, float32, []*Post, *misc.GeoRecord, error) {
	// Info for last 30 posts
	// https://api.instagram.com/v1/users/15930549/media/recent/?client_id=5941ed0c28874764a5d86fb47984aceb&count=20
	var latestGeo *misc.GeoRecord

	posts := []*Post{}

	endpoint := fmt.Sprintf(postUrl, cfg.Instagram.Endpoint, id, cfg.Instagram.ClientId)

	var media UserPost
	err := misc.Request("GET", endpoint, "", &media)
	if err != nil {
		return 0, 0, posts, latestGeo, err
	}

	if media.Meta.Code != 200 {
		return 0, 0, posts, latestGeo, ErrCode
	}

	if media.Data == nil || len(media.Data) == 0 {
		return 0, 0, posts, latestGeo, ErrUnknown
	}

	var (
		likes, comments float32
		published       int64
	)

	// Last 30 posts
	for _, post := range media.Data {
		published, err = strconv.ParseInt(post.Published, 10, 64)
		if err != nil {
			continue
		}

		p := &Post{
			Id:          post.Id,
			Published:   int32(published),
			Hashtags:    post.Tags,
			PostURL:     post.URL,
			LastUpdated: int32(time.Now().Unix()),
		}

		if post.Comments != nil {
			comments += post.Comments.Count
			p.Comments = post.Comments.Count
		}

		if post.Likes != nil {
			likes += post.Likes.Count
			p.Likes = post.Likes.Count
		}

		if post.Location != nil && post.Location.Latitude != 0 && post.Location.Longtitude != 0 {
			geo := misc.GetGeoFromCoords(post.Location.Latitude, post.Location.Longtitude, published)
			p.Location = geo
			if latestGeo == nil || published > latestGeo.Timestamp {
				latestGeo = geo
			}
		}

		if post.Caption != nil {
			p.Caption = post.Caption.Msg
		}

		posts = append(posts, p)
	}

	return likes / postCount, comments / postCount, posts, latestGeo, nil
}

type BasicUser struct {
	Meta *Meta     `json:"meta"`
	Data *UserData `json:"data"`
}

type UserData struct {
	Website string  `json:"website"`
	Name    string  `json:"username"`
	Id      string  `json:"id"`
	Counts  *Counts `json:"counts"`
}

type Counts struct {
	Followers float32 `json:"followed_by"`
}

func getUserInfo(id string, cfg *config.Config) (flw float32, url string, err error) {
	// followers: https://api.instagram.com/v1/users/15930549/?client_id=5941ed0c28874764a5d86fb47984aceb&count=25
	endpoint := fmt.Sprintf(followersUrl, cfg.Instagram.Endpoint, id, cfg.Instagram.ClientId)
	var user BasicUser
	err = misc.Request("GET", endpoint, "", &user)
	if err != nil {
		return
	}

	if user.Meta.Code != 200 {
		return
	}

	if user.Data != nil {
		url = user.Data.Website
		if user.Data.Counts != nil {
			flw = float32(user.Data.Counts.Followers)
		}
	}

	return
}

type PostById struct {
	Meta *Meta     `json:"meta"`
	Data *PostData `json:"data"`
}
