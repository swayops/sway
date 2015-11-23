package youtube

import "github.com/swayops/sway/internal/config"

type Post struct {
	Id          string
	Title       string
	Description string
	Published   int32 // Epoch ts

	PostURL string // Link to the post

	// Stats
	Views    float32
	Likes    float32
	Dislikes float32
	Comments float32
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	pt.Likes = 0
	pt.Dislikes = 0
	pt.Views = 0
	pt.Comments = 0
	return nil
}
