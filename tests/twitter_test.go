package tests

import (
	"log"
	"testing"

	"github.com/swayops/sway/config"
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
	inf, err := influencer.New(twId, "", "", "", "m", "FAKEAGENCY", []string{"groupId"}, 0, DefaultGeo, cfg)
	if err != nil {
		t.Fatal("Error when initializing twitter", err)
	}

	tw := inf.Twitter
	t.Logf("AvgRetweets: %v, AvgLikes: %v, Followers: %v, LatestPosts: %v", tw.AvgRetweets, tw.AvgLikes, uint(tw.Followers), len(tw.LatestTweets))
	log.Println("POST URL", tw.LatestTweets[0].PostURL())
	if v := tw.AvgRetweets; v < 500 {
		t.Fatal("AvgRetweets don't match! Expected > 500.. Got: ", v)
	}

	if v := tw.AvgLikes; v < 2000 {
		t.Fatal("AvgLikes don't match! Expected > 2000.. Got: ", v)
	}

	if v := tw.Followers; v < 36e6 {
		t.Fatal("Followers don't match! Expected > 3mil, because the world is broken.. Got: ", uint(v))
	}

	if err = tw.LatestTweets[0].UpdateData(cfg); err != nil {
		t.Fatal(err)
	}
}
