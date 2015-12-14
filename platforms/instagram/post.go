package instagram

import (
	"errors"
	"fmt"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	postInfoUrl = "%smedia/%s?client_id=%s"
)

var (
	ErrBadResponse = errors.New(`Empty data response from insta post!`)
)

type Post struct {
	Id       string
	Caption  string
	Hashtags []string

	PostURL string // Link to the post

	Published int32 //epoch ts

	Location *misc.GeoRecord

	// Stats
	Likes    float32
	Comments float32

	// Type
	Type string // "photo" or "video"
}

type DataByPost struct {
	Data *PostStats `json:"data"`
}

type PostStats struct {
	Comments *PostComments `json:"comments"`
	Likes    *PostLikes    `json:"likes"`
}

type PostComments struct {
	Count float32 `json:"count"`
}

type PostLikes struct {
	Count float32 `json:"count"`
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	endpoint := fmt.Sprintf(postInfoUrl, cfg.Instagram.Endpoint, pt.Id, cfg.Instagram.ClientId)

	var post DataByPost
	err := misc.Request("GET", endpoint, "", &post)
	if err != nil {
		return err
	}

	if post.Data == nil {
		return ErrBadResponse
	}

	if post.Data.Comments != nil {
		pt.Comments = post.Data.Comments.Count
	}

	if post.Data.Likes != nil {
		pt.Likes = post.Data.Likes.Count
	}

	return nil
}
