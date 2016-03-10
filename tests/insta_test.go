package tests

import (
	"log"
	"testing"
	"time"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestInstagram(t *testing.T) {
	// Complete once API has been built out

	cfg, err := config.New("./config.sample.json")
	if err != nil {
		log.Println("Config error", err)
	}

	// Initialize Influencer test
	instaId := "kimkardashian"
	inf, err := influencer.New("", instaId, "", "", "m", "FAKEAGENCY", []string{"groupId"}, 0, DefaultGeo, cfg)
	if err != nil {
		t.Error("Error when initializing insta", err)
	}

	if inf.Instagram.Followers < 1000000 {
		t.Error("Followers don't match! Expected > 1000000.. Got: ", inf.Instagram.Followers)
	}

	if inf.Instagram.AvgComments < 100 {
		t.Error("Comments don't match! Expected > 100.. Got: ", inf.Instagram.AvgComments)
	}

	if inf.Instagram.AvgLikes < 100 {
		t.Error("Likes don't match! Expected > 100.. Got: ", inf.Instagram.AvgLikes)
	}

	if inf.Instagram.UserId != "18428658" {
		t.Error("Incorrect user id. Expected: 18428658.. Got:", inf.Instagram.UserId)
	}

	if len(inf.Instagram.LatestPosts) == 0 {
		t.Error("Empty number of posts")
	}

	// Hacky test
	old := inf.Instagram.LatestPosts[0].Likes
	time.Sleep(20 * time.Second)
	inf.Instagram.LatestPosts[0].UpdateData(cfg)
	if old == inf.Instagram.LatestPosts[0].Likes {
		t.Error("Should have new likes data!")
	}

	// Update Influencer
	err = inf.Instagram.UpdateData(cfg)
	if err != nil {
		t.Error("Failed to update data")
	}

	if len(inf.Instagram.LatestPosts) != 0 {
		t.Error("Got new posts within a second.. not right!")
	}

	err = inf.NewInsta("randomdudewhodoesnthaveinsta123", cfg)
	if err == nil {
		t.Error("Expected error for randomdudewhodoesnthaveinsta123")
	}

	if inf.Instagram.UserName != "kimkardashian" {
		t.Error("Insta changed on bad user name")
	}

}
