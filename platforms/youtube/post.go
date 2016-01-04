package youtube

import (
	"encoding/json"

	"github.com/missionMeteora/iodb"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

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

func (pt *Post) UpdateData(db *iodb.DB, cfg *config.Config) (err error) {
	if rc := misc.GetPlatformCache(db, misc.PlatformYoutube, pt.Id); rc != nil {
		defer rc.Close()
		var post Post
		if err = json.NewDecoder(rc).Decode(&post); err != nil {
			return
		}
		*pt = post
		return
	}
	views, likes, dislikes, comments, err := getVideoStats(pt.Id, cfg)
	if err != nil {
		return err
	}

	pt.Likes = likes
	pt.Dislikes = dislikes
	pt.Views = views
	pt.Comments = comments
	j, _ := json.Marshal(pt)

	return misc.PutPlatformCache(db, misc.PlatformYoutube, pt.Id, j, misc.DefaultCacheDuration)
}
