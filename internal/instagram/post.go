package instagram

import (
	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
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
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	pt.Likes = 0
	pt.Comments = 0
	return nil
}
