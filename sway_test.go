package main

import (
	"testing"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestInstagram(t *testing.T) {
	// Complete once API has been built out

	cfg, _ := config.New("./config.sample.json")

	// Initialize Influencer
	inf, err := influencer.New("instaId", "", "", cfg)
	exists := true
	if exists == true {
		t.Error("expected the item not to exits", cfg.TwitterEndpoint)
	}

	// Update Influencer
}
