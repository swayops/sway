package facebook

import (
	"fmt"
	"log"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	postUrl      = "%s%s/posts?access_token=%s|%s"
	likesUrl     = "%s%s/likes?access_token=%s|%s&summary=true"
	commentsUrl  = "%s%s/comments?access_token=%s|%s&summary=true"
	sharesUrl    = "%s?id=%s"
	followersUrl = "%s%s?access_token=%s|%s&fields=likes"
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
	Count float64 `json:"total_count"`
}

//

type SharesData struct {
	Shares *Share `json:"shares"`
	Type   string `json:"type"`
}

type Share struct {
	Count float64 `json:"count"`
}

//

func getBasicInfo(id string, cfg *config.Config) (likes, comments, shares float64, fbPosts []*Post, err error) {
	//https://graph.facebook.com/dayoutubeguy/posts?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0
	// gets last 20 posts
	endpoint := fmt.Sprintf(postUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var posts PostData
	err = misc.Request("GET", endpoint, "", &posts)
	if err != nil || len(posts.Data) == 0 {
		log.Println("Error extracting posts", endpoint)
		return
	}

	var filtered []*Data
	if len(posts.Data) > 10 {
		filtered = posts.Data[0:10]
	} else {
		filtered = posts.Data
	}

	for _, p := range filtered {
		fbPost := &Post{
			Id:          p.Id,
			Caption:     p.Caption,
			Published:   p.Published,
			LastUpdated: int32(time.Now().Unix()),
			PostURL:     getPostUrl(p.Id),
		}

		if lk, lkErr := getLikes(p.Id, cfg); lkErr == nil {
			fbPost.Likes = lk
			fbPost.LikesDelta = lk
			likes += lk
		} else {
			err = lkErr
			return
		}

		if cm, cmErr := getComments(p.Id, cfg); cmErr == nil {
			fbPost.Comments = cm
			fbPost.CommentsDelta = cm
			comments += cm
		} else {
			err = cmErr
			return
		}

		if sh, pType, shErr := getShares(p.Id, cfg); shErr == nil {
			fbPost.Shares = sh
			fbPost.SharesDelta = sh
			fbPost.Type = pType
			shares += sh
		} else {
			err = shErr
			return
		}

		fbPosts = append(fbPosts, fbPost)
	}

	total := float64(len(fbPosts))
	likes = likes / total
	comments = comments / total
	shares = shares / total

	return
}

func getLikes(id string, cfg *config.Config) (lk float64, err error) {
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

func getComments(id string, cfg *config.Config) (cm float64, err error) {
	// https://graph.facebook.com/v2.5/212270682131283_1171691606189181/comments?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&summary=true
	endpoint := fmt.Sprintf(commentsUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var comments PostData
	err = misc.Request("GET", endpoint, "", &comments)
	if err != nil || comments.Summary == nil {
		log.Println("Error extracting comments", err)
		return
	}

	cm = comments.Summary.Count
	return
}

func getShares(id string, cfg *config.Config) (shares float64, pType string, err error) {
	// https://graph.facebook.com/v2.5/212270682131283_1171691606189181/comments?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&summary=true
	endpoint := fmt.Sprintf(sharesUrl, cfg.Facebook.Endpoint, id)
	var post SharesData
	err = misc.Request("GET", endpoint, "", &post)
	if err != nil {
		log.Println("Error extracting shares", err, endpoint)
		return
	}
	pType = post.Type
	if post.Shares != nil {
		// Some post types do not have shares.. i.e. "link" and "shared_story"
		shares = post.Shares.Count
	}
	return
}

type FollowerData struct {
	Likes float64 `json:"likes"`
}

func getFollowers(id string, cfg *config.Config) (fl float64, err error) {
	//https://graph.facebook.com/v2.5/cocacola?access_token=160153604335761|d306e3e3bbf5995f18b8ff8507ff4cc0&fields=likes
	endpoint := fmt.Sprintf(followersUrl, cfg.Facebook.Endpoint, id, cfg.Facebook.Id, cfg.Facebook.Secret)
	var data FollowerData
	err = misc.Request("GET", endpoint, "", &data)
	if err != nil {
		log.Println("Error extracting followers", err)
		return
	}
	fl = data.Likes
	return
}

const (
	postURL = "http://www.facebook.com/%s"
)

func getPostUrl(id string) string {
	return fmt.Sprintf(postURL, id)
}
