package instagram

import (
	"errors"
	"fmt"
	"time"

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
	Id       string   `json:"id"`
	Caption  string   `json:"caption,omitempty"`
	Hashtags []string `json:"hashtags,omitempty"`

	PostURL string `json:"postUrl,omitempty"` // Link to the post

	Published int32 `json:"published,omitempty"` //epoch ts

	Location *misc.GeoRecord `json:"location,omitempty"`

	// Stats
	Likes      float32 `json:"likes,omitempty"`
	LikesDelta float32 `json:"lDelta,omitempty"`

	Comments      float32 `json:"comments,omitempty"`
	CommentsDelta float32 `json:"cDelta,omitempty"`

	// Type
	Type string `json:"type,omitempty"` // "photo" or "video"

	LastUpdated int32 `json:"lastUpdated,omitempty"`
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
		pt.CommentsDelta = post.Data.Comments.Count - pt.Comments
		pt.Comments = post.Data.Comments.Count
	}

	if post.Data.Likes != nil {
		pt.LikesDelta = post.Data.Likes.Count - pt.Likes
		pt.Likes = post.Data.Likes.Count
	}

	pt.LastUpdated = int32(time.Now().Unix())

	return nil
}
