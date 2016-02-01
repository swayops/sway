package tests

import (
	"log"
	"testing"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestFacebook(t *testing.T) {
	// Complete once API has been built out

	cfg, err := config.New("./config.sample.json")
	if err != nil {
		log.Println("Config error", err)
	}

	// Initialize Influencer test
	fbId := "KimKardashian"
	inf, err := influencer.New("", "", fbId, "", "", "CAT1", "FAKEAGENCY", "m", 0, nil, cfg)
	if err != nil {
		t.Error("Error when initializing insta", err)
	}

	if inf.Facebook.Followers < 1000000 {
		t.Error("Followers don't match! Expected > 1000000.. Got: ", inf.Facebook.Followers)
	}

	if inf.Facebook.AvgComments < 100 {
		t.Error("Comments don't match! Expected > 100.. Got: ", inf.Facebook.AvgComments)
	}

	if inf.Facebook.AvgLikes < 100 {
		t.Error("Likes don't match! Expected > 100.. Got: ", inf.Facebook.AvgLikes)
	}

	if inf.Facebook.AvgShares < 100 {
		t.Error("Likes don't match! Expected > 100.. Got: ", inf.Facebook.AvgLikes)
	}

	if inf.Facebook.Id != fbId {
		t.Error("Incorrect user id. Expected: JustinBieber.. Got:", inf.Facebook.Id)
	}

	if len(inf.Facebook.LatestPosts) == 0 {
		t.Error("Empty number of posts")
	}

	// Hacky test
	old := inf.Facebook.LatestPosts[0].Likes
	time.Sleep(20 * time.Second)
	inf.Facebook.LatestPosts[0].UpdateData(cfg)
	if old == inf.Facebook.LatestPosts[0].Likes {
		t.Error("Should have new likes data!")
	}
}
