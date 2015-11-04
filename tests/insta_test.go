package tests

import (
	"log"
	"testing"

	"github.com/swayops/sway/internal/config"
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
	inf, err := influencer.New("", instaId, "", cfg)
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

	// Update Influencer
	err = inf.NewInsta("randomdudewhodoesnthaveinsta123", cfg)
	if err == nil {
		t.Error("Expected error for randomdudewhodoesnthaveinsta123")
	}

	if inf.Instagram.UserName != "kimkardashian" {
		t.Error("Insta changed on bad user name")
	}
}
