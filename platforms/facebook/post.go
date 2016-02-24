package facebook

import (
	"strings"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Post struct {
	Id        string `json:"id"`
	Caption   string `json:"caption,omitempty"`
	Published FbTime `json:"published,omitempty"`

	// Stats
	Likes    float32 `json:"likes,omitempty"`
	Shares   float32 `json:"shares,omitempty"`
	Comments float32 `json:"comments,omitempty"`

	// Type
	Type string `json:"type,omitempty"` // "video", "photo", "shared_story", "link"

	LastUpdated int32  `json:"lastUpdated,omitempty"`
	PostURL     string `json:"postURL,omitempty"`
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	// If we have already updated within the last 12 hours, skip!
	if misc.WithinLast(pt.LastUpdated, 12) {
		return nil
	}

	if lk, err := getLikes(pt.Id, cfg); err == nil {
		pt.Likes = lk
	} else {
		return err
	}

	if cm, err := getComments(pt.Id, cfg); err == nil {
		pt.Comments = cm
	} else {
		return err
	}

	if sh, _, err := getShares(pt.Id, cfg); err == nil {
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
