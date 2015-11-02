package instagram

import (
	"errors"
	"fmt"
	"strings"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

const (
	searchesUrl = "%susers/search?q=%s&client_id=%s"
)

var (
	ErrCode    = errors.New(`Non-200 Instagram Status Code`)
	ErrUnknown = errors.New(`User not found!`)
)

type UserSearch struct {
	Meta struct {
		Code int `json:"code"`
	} `json:"meta"`
	Data []*Data `json:"data"`
}

type Data struct {
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

func getLikes(id string, cfg *config.Config) (int, error) {
	// https://api.instagram.com/v1/users/15930549/media/recent/?client_id=5941ed0c28874764a5d86fb47984aceb&count=20
	// followers: https://api.instagram.com/v1/users/15930549/?client_id=5941ed0c28874764a5d86fb47984aceb&count=25
	return 0, nil
}
