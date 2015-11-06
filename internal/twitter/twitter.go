package twitter

type Twitter struct {
	Id string

	RetweetsPerPost float32
	Followers       float32
	FollowerDelta   float32 // Follower delta since last UpdateData run

	LastLocation string //TBD
}

func New(id, endpoint string) (*Twitter, error) {
	tw := &Twitter{
		Id: id,
	}
	err := tw.UpdateData(endpoint)
	return tw, err
}

func (tw *Twitter) UpdateData(endpoint string) error {
	// Used by an eventual ticker to update stats
	if tw.Id != "" {
		if rt, err := getRetweets(tw.Id, endpoint); err == nil {
			tw.RetweetsPerPost = rt
		} else {
			return err
		}

		if fl, err := getFollowers(tw.Id, endpoint); err == nil {
			tw.Followers = fl
		} else {
			return err
		}
	}
	return nil
}

func getRetweets(id, endpoint string) (int, error) {
	return 0, nil
}

func getFollowers(id, endpoint string) (int, error) {
	return 0, nil
}
