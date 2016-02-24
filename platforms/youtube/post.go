package youtube

import (
	"strings"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Post struct {
	Id          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"desc,omitempty"`
	Published   int32  `json:"published,omitempty"` // Epoch ts

	PostURL string `json:"url,omitempty"` // Link to the post

	// Stats
	Views    float32 `json:"views,omitempty"`
	Likes    float32 `json:"likes,omitempty"`
	Dislikes float32 `json:"dislikes,omitempty"`
	Comments float32 `json:"comments,omitempty"`

	LastUpdated int32 `json:"lastUpdated,omitempty"`
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	// If we have already updated within the last 12 hours, skip!
	if misc.WithinLast(pt.LastUpdated, 12) {
		return nil
	}

	views, likes, dislikes, comments, err := getVideoStats(pt.Id, cfg)
	if err != nil {
		return err
	}

	pt.Likes = likes
	pt.Dislikes = dislikes
	pt.Views = views
	pt.Comments = comments
	pt.LastUpdated = int32(time.Now().Unix())

	return nil
}

func (pt *Post) Hashtags() []string {
	tags := []string{}
	parts := strings.Split(pt.Description, " ")
	for _, p := range parts {
		if len(p) > 1 && string(p[0]) == "#" {
			tags = append(tags, strings.ToLower(p[1:]))
		}
	}
	return tags
}
