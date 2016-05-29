package facebook

import (
	"strings"
	"time"

	"github.com/swayops/sway/config"
)

type Post struct {
	Id        string `json:"id"`
	Caption   string `json:"caption,omitempty"`
	Published FbTime `json:"published,omitempty"`

	// Stats
	Likes      float64 `json:"likes,omitempty"`
	LikesDelta float64 `json:"lDelta,omitempty"`

	Shares      float64 `json:"shares,omitempty"`
	SharesDelta float64 `json:"shDelta,omitempty"`

	Comments      float64 `json:"comments,omitempty"`
	CommentsDelta float64 `json:"cDelta,omitempty"`

	// Type
	Type string `json:"type,omitempty"` // "video", "photo", "shared_story", "link"

	LastUpdated int32  `json:"lastUpdated,omitempty"`
	PostURL     string `json:"postURL,omitempty"`
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	// // If the post is more than 4 days old AND
	// // it has been updated in the last week, SKIP!
	// // i.e. only update old posts once a week
	// if !misc.WithinLast(int32(pt.Published.Unix()), 24*4) && misc.WithinLast(pt.LastUpdated, 24*7) {
	// 	return nil
	// }

	// // If we have already updated within the last 12 hours, skip!
	// if misc.WithinLast(pt.LastUpdated, 12) {
	// 	return nil
	// }

	if lk, err := getLikes(pt.Id, cfg); err == nil {
		pt.LikesDelta = pt.Likes - lk
		pt.Likes = lk
	} else {
		return err
	}

	if cm, err := getComments(pt.Id, cfg); err == nil {
		pt.CommentsDelta = pt.Comments - cm
		pt.Comments = cm
	} else {
		return err
	}

	if sh, _, err := getShares(pt.Id, cfg); err == nil {
		pt.SharesDelta = pt.Shares - sh
		pt.Shares = sh
	} else {
		return err
	}

	pt.LastUpdated = int32(time.Now().Unix())

	return nil
}

func (pt *Post) Hashtags() []string {
	tags := []string{}
	parts := strings.Split(pt.Caption, " ")
	for _, p := range parts {
		if len(p) > 1 && string(p[0]) == "#" {
			tags = append(tags, strings.ToLower(p[1:]))
		}
	}
	return tags
}
