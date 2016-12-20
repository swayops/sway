package youtube

import (
	"strings"
	"time"

	"github.com/swayops/sway/config"
)

type Post struct {
	Id          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"desc"`
	Published   int32  `json:"published"` // Epoch ts

	PostURL string `json:"url"` // Link to the post

	// Stats
	Views      float64 `json:"views"`
	ViewsDelta float64 `json:"vDelta"`

	Likes      float64 `json:"likes"`
	LikesDelta float64 `json:"lDelta"`

	Dislikes      float64 `json:"dislikes"`
	DislikesDelta float64 `json:"dlDelta"`

	Comments      float64 `json:"comments"`
	CommentsDelta float64 `json:"cDelta"`

	LastUpdated int32 `json:"lastUpdated"`
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

	views, likes, dislikes, comments, desc, err := getVideoStats(pt.Id, cfg)
	if err != nil {
		return err
	}
	pt.Description = desc

	pt.LikesDelta = likes - pt.Likes
	pt.Likes = likes

	pt.DislikesDelta = dislikes - pt.Dislikes
	pt.Dislikes = dislikes

	pt.ViewsDelta = views - pt.Views
	pt.Views = views

	pt.CommentsDelta = comments - pt.Comments
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

func (pt *Post) Clear() {
	pt.LikesDelta, pt.ViewsDelta, pt.DislikesDelta, pt.CommentsDelta = 0, 0, 0, 0
}
