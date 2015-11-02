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
	inf, err := influencer.New("melissamolinaro", "melissamolinaro", "melissamolinaro", cfg)
	exists := true
	if exists == true {
		t.Error("expected the item not to exits", inf, err)
	}

	log.Println("Insta User ID", inf.Instagram.UserId)

	// Update Influencer
}
