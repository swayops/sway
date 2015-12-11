package tests

import (
	"log"
	"testing"

	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/internal/influencer"
)

func TestTumblr(t *testing.T) {
	// Complete once API has been built out

	cfg, err := config.New("./config.sample.json")
	if err != nil {
		log.Println("Config error", err)
	}

	// Initialize Influencer test
	id := "kropotkindersurprise.tumblr.com" // I may hate the bitch but sadly she's a huge star :(
	inf, err := influencer.New("", "", "", "", id, cfg)
	if err != nil {
		t.Fatal("Error when initializing tumblr", err)
	}

	tr := inf.Tumblr
	t.Logf("AvgReblogs: %v, AvgLikes: %v, LatestPosts: %v", tr.AvgReblogs, tr.AvgLikes, len(tr.LatestPosts))

	log.Println(tr.LatestPosts[0].UpdateData(tr, cfg))
	// if v := tw.AvgRetweets; v < 500 {
	// 	t.Fatal("AvgRetweets don't match! Expected > 500.. Got: ", v)
	// }

	// if v := tw.AvgLikes; v < 2000 {
	// 	t.Fatal("AvgLikes don't match! Expected > 2000.. Got: ", v)
	// }

	// if v := tw.Followers; v < 36e6 {
	// 	t.Fatal("Followers don't match! Expected > 3mil, because the world is broken.. Got: ", uint(v))
	// }

	// if err = tw.LatestTweets[0].UpdateData(cfg); err != nil {
	// 	t.Fatal(err)
	// }
}
