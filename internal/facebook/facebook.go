package facebook

import (
	"time"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/misc"
)

type Facebook struct {
	Id string

	LikesPerPost    float32
	CommentsPerPost float32
	Followers       float32 // float32 for GetScore equation
	FollowerDelta   float32 // Follower delta since last UpdateData run

	LastLocation misc.GeoRecord

	LastUpdated int64   // Epoch timestamp in seconds
	PostsSince  []*Post // Posts since last update.. will later check these for deal satisfaction

	Score float32
}

type Post struct {
	Id        string
	Content   string
	Mentions  []string
	Timestamp int32

	// Stats
	Likes    int32
	Shares   int32
	Comments int32
}

func New(id string, cfg *config.Config) (*Facebook, error) {
	fb := &Facebook{
		Id: id,
	}
	err := fb.UpdateData(cfg.FbEndpoint)
	return fb, err
}

func (fb *Facebook) UpdateData(endpoint string) error {
	// Used by an eventual ticker to update stats
	if fb.Id != "" {
		if likes, err := getLikes(fb.Id, endpoint); err == nil {
			fb.LikesPerPost = likes
		} else {
			return err
		}

		if cm, err := getComments(fb.Id, endpoint); err == nil {
			fb.CommentsPerPost = cm
		} else {
			return err
		}

		if fl, err := getFollowers(fb.Id, endpoint); err == nil {
			fb.Followers = fl
		} else {
			return err
		}
		fb.PostsSince = getPosts(fb.LastUpdated)
		fb.LastUpdated = time.Now().Unix()
	}
	return nil
}

func getLikes(id, endpoint string) (float32, error) {
	return 0, nil
}

func getComments(id, endpoint string) (float32, error) {
	return 0, nil
}

func getFollowers(id, endpoint string) (float32, error) {
	return 0, nil
}

func getPosts(last int64) []*Post {
	return nil
}

func GetStatsByPost(id string) *Post {
	// Each package has this function.. so we can update stats for deal posts
	// Should take in a post Id and return all post stats
	return nil
}
