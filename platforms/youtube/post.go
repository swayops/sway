package youtube

import "github.com/swayops/sway/config"

type Post struct {
	Id          string `json:"id"`
	Title       string `json:"title, omitempty"`
	Description string `json:"desc, omitempty"`
	Published   int32  `json:"published, omitempty"` // Epoch ts

	PostURL string `json:"url, omitempty"` // Link to the post

	// Stats
	Views    float32 `json:"views, omitempty"`
	Likes    float32 `json:"likes, omitempty"`
	Dislikes float32 `json:"dislikes, omitempty"`
	Comments float32 `json:"comments, omitempty"`
}

func (pt *Post) UpdateData(cfg *config.Config) error {
	views, likes, dislikes, comments, err := getVideoStats(pt.Id, cfg)
	if err != nil {
		return err
	}

	pt.Likes = likes
	pt.Dislikes = dislikes
	pt.Views = views
	pt.Comments = comments
	return nil
}
