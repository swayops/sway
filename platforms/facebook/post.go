package facebook

import "github.com/swayops/sway/config"

type Post struct {
	Id        string
	Caption   string
	Published FbTime

	// Stats
	Likes    float32
	Shares   float32
	Comments float32

	// Type
	Type string // "video", "photo", "shared_story", "link"
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
