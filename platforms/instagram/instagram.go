package instagram

import (
	"errors"
	"log"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
)

// AUTH:
// https://api.instagram.com/oauth/authorize/?client_id={{CLIENT_ID}}&redirect_uri=http://lol:8080&response_type=token&scope=basic+public_content
var (
	ErrEligible = errors.New("Instagram account is not eligible")
)

type Instagram struct {
	UserName string `json:"userName"`
	UserId   string `json:"userId"`

	FullName string `json:"fullName"`
	Bio      string `json:"bio"`

	AvgLikes      float64 `json:"avgLikes,omitempty"`    // Per post
	AvgComments   float64 `json:"avgComments,omitempty"` // Per post
	Followers     float64 `json:"followers,omitempty"`
	FollowerDelta float64 `json:"fDelta,omitempty"` // Follower delta since last UpdateData run

	LastLocation *geo.GeoRecord `json:"geo,omitempty"` // All locations since last update

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	Images []string `json:"images,omitempty"` // List of extracted image urls from last UpdateData run

	LinkInBio string `json:"link,omitempty"`

	ProfilePicture string `json:"profile_picture,omitempty"`
}

func New(name string, cfg *config.Config) (*Instagram, error) {
	log.Println("Entered", name, time.Now().Unix())
	userId, err := getUserIdFromName(name, cfg)
	if err != nil {
		return nil, err
	}
	log.Println("Checkpoint 1", name, time.Now().Unix())

	in := &Instagram{
		UserName: name,
		UserId:   userId,
	}

	err = in.UpdateData(cfg, cfg.Sandbox)
	if err != nil {
		return nil, err
	}
	log.Println("Checkpoint 2", name, time.Now().Unix())

	if in.Followers < 10 {
		return nil, ErrEligible
	}
	log.Println("Exited", name, time.Now().Unix())

	return in, nil
}

func (in *Instagram) UpdateData(cfg *config.Config, savePosts bool) error {
	// Used by an eventual ticker to update stats

	// If we already updated in the last 21-26 hours, skip
	// if misc.WithinLast(in.LastUpdated, misc.Random(21, 26)) {
	// 	return nil
	// }
	log.Println("Getting user info", in.UserName, time.Now().Unix())
	if fl, link, dp, bio, err := getUserInfo(in.UserId, cfg); err == nil {
		if in.Followers > 0 {
			// Make sure this isn't first run
			in.FollowerDelta = (fl - in.Followers)
		}
		in.Followers = fl
		in.LinkInBio = link
		in.ProfilePicture = dp
		in.Bio = bio
	} else {
		return err
	}
	log.Println("Getting post info", in.UserName, time.Now().Unix())

	if pInfo, err := getPostInfo(in.UserId, cfg); err == nil {
		in.AvgLikes = pInfo.Likes
		in.AvgComments = pInfo.Comments
		in.Images = pInfo.Images

		// Latest posts are only used when there is an active deal!
		if savePosts {
			in.LatestPosts = pInfo.Posts
		} else {
			in.LatestPosts = nil
		}

		if pInfo.Geo != nil {
			in.LastLocation = pInfo.Geo
		}

		if pInfo.Name != "" {
			in.FullName = pInfo.Name
		}
	} else {
		return err
	}
	log.Println("Exiting user info", in.UserName, time.Now().Unix())

	in.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (in *Instagram) GetScore() float64 {
	return (in.Followers * 3) + (in.AvgComments * 2) + (in.AvgLikes)
}

func (in *Instagram) GetProfileURL() string {
	return "https://www.instagram.com/" + in.UserName
}
