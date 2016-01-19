package facebook

import "github.com/swayops/sway/config"

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
}

func (pt *Post) UpdateData(cfg *config.Config) error {
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

	return nil
}
