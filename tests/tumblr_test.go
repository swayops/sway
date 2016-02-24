package tests

// func TestTumblr(t *testing.T) {
// 	// Complete once API has been built out

// 	cfg, err := config.New("./config.sample.json")
// 	if err != nil {
// 		log.Println("Config error", err)
// 	}

// 	// Initialize Influencer test
// 	id := "kropotkindersurprise.tumblr.com" // I may hate the bitch but sadly she's a huge star :(
// 	inf, err := influencer.New("", "", "", "", id, "m", "FAKEAGENCY", []string{"groupId"}, 0, DefaultGeo, cfg)
// 	if err != nil {
// 		t.Fatal("Error when initializing tumblr", err)
// 	}

// 	tr := inf.Tumblr
// 	t.Logf("AvgReblogs: %v, AvgLikes: %v, LatestPosts: %v", tr.AvgReblogs, tr.AvgLikes, len(tr.LatestPosts))

// 	if err := tr.LatestPosts[0].UpdateData(tr, cfg); err != nil {
// 		t.Fatal(err)
// 	}
// }
