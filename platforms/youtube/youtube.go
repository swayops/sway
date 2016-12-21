package youtube

import (
	"errors"
	"time"

	"github.com/swayops/sway/config"
)

type YouTube struct {
	UserName string `json:"userName"`
	UserId   string `json:"id"`

	AvgLikes    float64 `json:"avgLikes,omitempty"`
	AvgDislikes float64 `json:"avgDislikes,omitempty"`

	AvgViews        float64 `json:"avgViews,omitempty"`
	AvgComments     float64 `json:"avgComments,omitempty"`
	Subscribers     float64 `json:"avgSubs,omitempty"`  // float64 for GetScore equation
	SubscriberDelta float64 `json:"subDelta,omitempty"` // Follower delta since last UpdateData run

	LastUpdated int32   `json:"lastUpdated,omitempty"` // Epoch timestamp in seconds
	LatestPosts []*Post `json:"posts,omitempty"`       // Posts since last update.. will later check these for deal satisfaction

	Images []string `json:"images,omitempty"` // List of extracted image urls from last UpdateData run

	ProfilePicture string `json:"profile_picture,omitempty"`
}

var (
	ErrEligible = errors.New("Youtube account is not eligible")
)

func New(userId string, cfg *config.Config) (*YouTube, error) {
	// Lets start off by assuming the channel id was passed in..
	yt := &YouTube{
		UserName: userId,
		UserId:   userId,
	}

	// Lets check if this id is a username..
	extractedUserId := getIdFromUsername(userId, cfg)
	if extractedUserId != "" {
		// It was a username that was passed in and we were able to get its id!
		yt.UserName = userId
		yt.UserId = extractedUserId
	}

	err := yt.UpdateData(cfg, cfg.Sandbox)
	if err != nil {
		return nil, ErrUnknown
	}

	if yt.Subscribers < 10 {
		return nil, ErrEligible
	}

	return yt, nil
}

func (yt *YouTube) UpdateData(cfg *config.Config, savePosts bool) error {
	// If we already updated in the last 21-26 hours, skip
	// if misc.WithinLast(yt.LastUpdated, misc.Random(21, 26)) {
	// 	return nil
	// }

	if views, comments, subs, dp, err := getUserStats(yt.UserId, cfg); err == nil {
		if yt.Subscribers > 0 {
			yt.SubscriberDelta = (subs - yt.Subscribers)
		}
		yt.AvgViews = views
		yt.AvgComments = comments
		yt.Subscribers = subs
		yt.ProfilePicture = dp
	} else {
		return ErrUnknown
	}

	p, lk, dlk, images, err := getPosts(yt.UserId, 5, cfg)
	if err != nil {
		return ErrUnknown
	}
	// Latest posts are only used when there is an active deal!
	if savePosts {
		yt.LatestPosts = p
	} else {
		yt.LatestPosts = nil
	}

	yt.Images = images
	yt.AvgLikes = lk
	yt.AvgDislikes = dlk

	yt.LastUpdated = int32(time.Now().Unix())
	return nil
}

func (yt *YouTube) GetScore() float64 {
	return (yt.Subscribers * 2.5) + (yt.AvgComments * 1.5) + float64(yt.AvgLikes) + float64(yt.AvgViews)
}
