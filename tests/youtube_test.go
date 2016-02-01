package tests

import (
	"log"
	"testing"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestYouTube(t *testing.T) {
	cfg, err := config.New("./config.sample.json")
	if err != nil {
		log.Println("Config error", err)
	}

	// Initialize Influencer test
	ytId := "JennaMarbles"
	inf, err := influencer.New("", "", "", ytId, "", "CAT1", "FAKEAGENCY", "m", 0, DefaultGeo, cfg)
	if err != nil {
		t.Error("Error when initializing insta", err)
	}

	if inf.YouTube.AvgLikes < 10 {
		t.Error("Likes don't match! Expected > 10.. Got: ", inf.YouTube.AvgLikes)
	}

	if inf.YouTube.AvgDislikes < 10 {
		t.Error("DisLikes don't match! Expected > 10.. Got: ", inf.YouTube.AvgDislikes)
	}

	if inf.YouTube.AvgViews < 10 {
		t.Error("Views don't match! Expected > 10.. Got: ", inf.YouTube.AvgViews)
	}

	if inf.YouTube.AvgComments < 10 {
		t.Error("Comments don't match! Expected > 10.. Got: ", inf.YouTube.AvgComments)
	}

	if inf.YouTube.Subscribers < 10 {
		t.Error("Subscribers don't match! Expected > 10.. Got: ", inf.YouTube.Subscribers)
	}

	if len(inf.YouTube.LatestPosts) != 10 {
		t.Error("Posts don't match! Expected 10.. Got: ", inf.YouTube.LatestPosts)
	}

	if inf.YouTube.LatestPosts[0].Likes == 0 {
		t.Error("Video likes don't match! Expected > 0.. Got: ", inf.YouTube.LatestPosts[0].Likes)
	}

	// Hacky test
	old := inf.YouTube.LatestPosts[0].Views
	time.Sleep(10 * time.Minute)
	inf.YouTube.LatestPosts[0].UpdateData(cfg)
	if old == inf.YouTube.LatestPosts[0].Views {
		t.Error("Should have new likes data!")
	}

	err = inf.YouTube.UpdateData(cfg)
	if err != nil {
		t.Error("Failed to update data")
	}

	if len(inf.YouTube.LatestPosts) != 0 {
		t.Error("Got new posts within a second.. not right!")
	}
}
