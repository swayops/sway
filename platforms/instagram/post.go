package instagram

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/misc"
)

const (
	postInfoUrl = "%smedia/%s?access_token=%s"
)

var (
	ErrBadResponse = errors.New(`Empty data response from insta post!`)
)

type Post struct {
	Id       string   `json:"id"`
	Caption  string   `json:"caption,omitempty"`
	Hashtags []string `json:"hashtags,omitempty"`

	PostURL   string `json:"postUrl,omitempty"`   // Link to the post
	Thumbnail string `json:"thumbnail,omitempty"` // Link to the post

	Published int32 `json:"published,omitempty"` //epoch ts

	Location *geo.GeoRecord `json:"location,omitempty"`

	// Stats
	Likes    float64 `json:"likes,omitempty"`
	Comments float64 `json:"comments,omitempty"`

	// Type
	Type string `json:"type,omitempty"` // "photo" or "video"

	LastUpdated int32 `json:"lastUpdated,omitempty"`
}

type DataByPost struct {
	Data *PostData `json:"data"`
	Meta *struct {
		ErrorType    string `json:"error_type,omitempty"`
		Code         int    `json:"code,omitempty"`
		ErrorMessage string `json:"error_message,omitempty"`
	} `json:"meta"`
}

func (pt *Post) UpdateData(cfg *config.Config) (error, error) {
	// // If the post is more than 4 days old AND
	// // it has been updated in the last week, SKIP!
	// // i.e. only update old posts once a week
	// if !misc.WithinLast(pt.Published, 24*4) && misc.WithinLast(pt.LastUpdated, 24*7) {
	// 	return nil
	// }

	// // If we have already updated within the last 12 hours, skip!
	// if misc.WithinLast(pt.LastUpdated, 12) {
	// 	return nil
	// }

	endpoint := fmt.Sprintf(postInfoUrl, cfg.Instagram.Endpoint, pt.Id, getToken(cfg.Instagram.AccessTokens))

	var post DataByPost
	err := misc.Request("GET", endpoint, "", &post)
	if err != nil {
		return nil, err
	}

	if post.Meta != nil && strings.ToLower(post.Meta.ErrorMessage) == "invalid media id" {
		return errors.New(post.Meta.ErrorMessage), nil
	}

	if post.Data == nil {
		return nil, ErrBadResponse
	}

	if post.Data.Comments != nil {
		pt.Comments = post.Data.Comments.Count
	}

	if post.Data.Likes != nil {
		pt.Likes = post.Data.Likes.Count
	}

	if post.Data.Images != nil && post.Data.Images.Resolution != nil && post.Data.Images.Resolution.URL != "" {
		pt.Thumbnail = post.Data.Images.Resolution.URL
	}

	pt.LastUpdated = int32(time.Now().Unix())

	return nil, nil
}
