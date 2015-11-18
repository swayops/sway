package tests

import (
	"log"
	"testing"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestTwitter(t *testing.T) {
	// Complete once API has been built out

	cfg, err := config.New("./config.sample.json")
	if err != nil {
		log.Println("Config error", err)
	}

	// Initialize Influencer test
	twId := "kimkardashian" // I may hate the bitch but sadly she's a huge star :(
	inf, err := influencer.New(twId, "", "", "", cfg)
	if err != nil {
		t.Error("Error when initializing insta", err)
	}

	tw := inf.Twitter
	t.Logf("AvgRetweets: %v, AvgLikes: %v, Followers: %v, LatestPosts: %v", tw.AvgRetweets, tw.AvgLikes, uint(tw.Followers), len(tw.LatestTweets))

	if v := tw.AvgRetweets; v < 800 {
		t.Error("AvgRetweets don't match! Expected > 800.. Got: ", v)
	}

	if v := tw.AvgLikes; v < 3000 {
		t.Error("AvgLikes don't match! Expected > 3000.. Got: ", v)
	}

	if v := tw.Followers; v < 36e6 {
		t.Error("Followers don't match! Expected > 3mil, because the world is broken.. Got: ", uint(v))
	}

}
