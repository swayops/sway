package instagram

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/missionMeteora/iodb"
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

func (pt *Post) UpdateData(db *iodb.DB, cfg *config.Config) (err error) {
	var post DataByPost
	if rc := misc.GetPlatformCache(db, misc.PlatformInstagram, pt.Id); rc != nil {
		defer rc.Close()
		if err = json.NewDecoder(rc).Decode(&post); err != nil {
			return
		}
	} else {
		endpoint := fmt.Sprintf(postInfoUrl, cfg.Instagram.Endpoint, pt.Id, cfg.Instagram.ClientId)

		if err = misc.Request("GET", endpoint, "", &post); err != nil {
			return err
		}
		if post.Data == nil {
			return ErrBadResponse
		}
		j, _ := json.Marshal(&post)
		if err = misc.PutPlatformCache(db, misc.PlatformInstagram, pt.Id, j, misc.DefaultCacheDuration); err != nil {
			return
		}
	}

	if post.Data.Comments != nil {
		pt.Comments = post.Data.Comments.Count
	}

	if post.Data.Likes != nil {
		pt.Likes = post.Data.Likes.Count
	}

	return
}
