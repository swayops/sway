package instagram

import (
	"errors"
	"fmt"
	"strings"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Meta struct {
	Code int `json:"code"`
}

const (
	postCount    = 30.0
	searchesUrl  = "%susers/search?q=%s&client_id=%s"
	followersUrl = "%susers/%s/?client_id=%s"
	postUrl      = "%susers/%s/media/recent/?client_id=%s&count=%s"
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
	// Location string `json:"location"` TBD.. Store as last location
	Comments *Comments `json:"comments"`
	Likes    *Likes    `json:"likes"`
}

type Comments struct {
	Count float32 `json:"count"`
}

type Likes struct {
	Count float32 `json:"count"`
}

func getPostInfo(id string, cfg *config.Config) (float32, float32, error) {
	// https://api.instagram.com/v1/users/15930549/media/recent/?client_id=5941ed0c28874764a5d86fb47984aceb&count=20
	endpoint := fmt.Sprintf(postUrl, cfg.Instagram.Endpoint, id, cfg.Instagram.ClientId, postCount)

	var media UserPost
	err := misc.Request("GET", endpoint, "", &media)
	if err != nil {
		return 0, 0, err
	}

	if media.Meta.Code != 200 {
		return 0, 0, ErrCode
	}

	if media.Data == nil || len(media.Data) == 0 {
		return 0, 0, ErrUnknown
	}

	var (
		likes, comments float32
	)

	for _, post := range media.Data {
		if post.Comments != nil {
			comments += post.Comments.Count
		}

		if post.Likes != nil {
			likes += post.Likes.Count
		}
	}
	return likes / postCount, comments / postCount, nil
}

type BasicUser struct {
	Meta *Meta     `json:"meta"`
	Data *UserData `json:"data"`
}

type UserData struct {
	Name   string  `json:"username"`
	Id     string  `json:"id"`
	Counts *Counts `json:"counts"`
}

type Counts struct {
	Followers float32 `json:"followed_by"`
}

func getFollowers(id string, cfg *config.Config) (flw float32, err error) {
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

	if user.Data.Counts != nil {
		flw = float32(user.Data.Counts.Followers)
	}

	return
}
