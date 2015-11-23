package facebook

import (
	"fmt"
	"log"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

const (
	postUrl      = "%sv2.5/%s/posts?access_token=%s|%s"
	likesUrl     = "%sv2.5/%s/likes?access_token=%s|%s&summary=true"
	commentsUrl  = "%sv2.5/%s/comments?access_token=%s|%s&summary=true"
	sharesUrl    = "%s?id=%s"
	followersUrl = "%s%s/insights/page_fans_country?access_token=%s|%s"
)

type PostData struct {
	Data    []*Data  `json:"data"`
	Summary *Summary `json:"summary"`
}

type Data struct {
	Id        string `json:"id"`
	Caption   string `json:"message"`
	Published FbTime `json:"created_time"`
}

type Summary struct {
	Count float32 `json:"total_count"`
}

//

type SharesData struct {
	Shares *Share `json:"shares"`
}

type Share struct {
	Count float32 `json:"count"`
}

//

func getBasicInfo(id string, cfg *config.Config) (likes, comments, shares float32, fbPosts []*Post, err error) {
	//https://graph.facebook.com/dayoutubeguy/posts?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0
	// gets last 25 posts
	endpoint := fmt.Sprintf(postUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var posts PostData
	err = misc.Request("GET", endpoint, "", &posts)
	if err != nil || len(posts.Data) == 0 {
		log.Println("Error extracting posts")
		return
	}

	for _, p := range posts.Data {
		fbPost := &Post{
			Id:        p.Id,
			Caption:   p.Caption,
			Published: p.Published,
		}

		if lk, err := getLikes(p.Id, cfg); err == nil {
			fbPost.Likes = lk
			likes += lk
		} else {
			continue
		}

		if cm, err := getComments(p.Id, cfg); err == nil {
			fbPost.Comments = cm
			comments += cm
		} else {
			continue
		}

		if sh, err := getShares(p.Id, cfg); err == nil {
			fbPost.Shares = sh
			shares += sh
		} else {
			continue
		}

		fbPosts = append(fbPosts, fbPost)
	}

	total := float32(len(fbPosts))
	likes = likes / total
	comments = comments / total
	shares = shares / total

	return
}

func getLikes(id string, cfg *config.Config) (lk float32, err error) {
	// https://graph.facebook.com/212270682131283_1171691606189181/likes?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&summary=true
	endpoint := fmt.Sprintf(likesUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var likes PostData
	err = misc.Request("GET", endpoint, "", &likes)
	if err != nil || likes.Summary == nil {
		log.Println("Error extracting likes", err)
		return
	}

	lk = likes.Summary.Count
	return
}

func getComments(id string, cfg *config.Config) (cm float32, err error) {
	// https://graph.facebook.com/212270682131283_1171691606189181/comments?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&summary=true
	endpoint := fmt.Sprintf(commentsUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var comments PostData
	err = misc.Request("GET", endpoint, "", &comments)
	if err != nil || comments.Summary == nil {
		log.Println("Error extracting likes", err)
		return
	}

	cm = comments.Summary.Count
	return
}

func getShares(id string, cfg *config.Config) (shares float32, err error) {
	// https://graph.facebook.com/212270682131283_1171691606189181/comments?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&summary=true
	endpoint := fmt.Sprintf(sharesUrl, cfg.Facebook.Endpoint, id)
	var post SharesData
	err = misc.Request("GET", endpoint, "", &post)
	if err != nil || post.Shares == nil {
		log.Println("Error extracting shares", err, endpoint)
		return
	}
	shares = post.Shares.Count
	return
}

type FollowerData struct {
	Data []*FanData `json:"data"`
}

type FanData struct {
	Values []*Value `json:"values"`
}

type Value struct {
	Countries map[string]float32 `json:"value"`
}

func getFollowers(id string, cfg *config.Config) (fl float32, err error) {
	//https://graph.facebook.com/cocacola/insights/page_fans_country?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0
	endpoint := fmt.Sprintf(followersUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var data FollowerData
	err = misc.Request("GET", endpoint, "", &data)
	if err != nil || len(data.Data) == 0 || len(data.Data[0].Values) == 0 {
		log.Println("Error extracting followers", err)
		return
	}

	for _, val := range data.Data[0].Values[0].Countries {
		fl += val
	}
	return
}
