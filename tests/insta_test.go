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
		log.Println("ERROR", err)
	}

	// Initialize Influencer test
	inf, err := influencer.New("", "kimkardashian", "", cfg)
	if err != nil {
		t.Error("Error when initializing insta", err)
	}

	if inf.Instagram.UserId != "18428658" {
		t.Error("Incorrect user id for", inf.Instagram.UserName)
	}

	if inf.Instagram.Followers < 1000000 {
		t.Error("Followers not retrieved", inf.Instagram.UserName)
	}

	if inf.Instagram.AvgComments < 100 {
		t.Error("Comments not retrieved", inf.Instagram.UserName)
	}

	if inf.Instagram.AvgLikes < 100 {
		t.Error("Likes not retrieved", inf.Instagram.UserName)
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
