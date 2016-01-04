package facebook

import (
	"encoding/json"

	"github.com/missionMeteora/iodb"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

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

func (pt *Post) UpdateData(db *iodb.DB, cfg *config.Config) (err error) {
	if rc := misc.GetPlatformCache(db, misc.PlatformFacebook, pt.Id); rc != nil {
		defer rc.Close()
		var post postData
		if err = json.NewDecoder(rc).Decode(&post); err != nil {
			return
		}
		pt.Likes, pt.Comments, pt.Shares = post.Likes, post.Comments, post.Shares
		return
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

	j, _ := json.Marshal(postData{pt.Likes, pt.Comments, pt.Shares})
	return misc.PutPlatformCache(db, misc.PlatformFacebook, pt.Id, j, misc.DefaultCacheDuration)
}

type postData struct {
	Likes, Comments, Shares float32
}
