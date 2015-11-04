package instagram

import "github.com/swayops/sway/internal/config"

type Instagram struct {
	UserName     string
	UserId       string
	AvgLikes     int
	AvgComments  int
	Followers    int
	LastLocation string //TBD
}

func New(name string, cfg *config.Config) (*Instagram, error) {
	userId, err := getUserIdFromName(name, cfg)
	if err != nil {
		return nil, err
	}

	in := &Instagram{
		UserName: name,
		UserId:   userId,
	}

	err = in.UpdateData(cfg)
	return in, err
}

func (in *Instagram) UpdateData(cfg *config.Config) error {
	// Used by an eventual ticker to update stats
	if in.UserId != "" {
		if fl, err := getFollowers(in.UserId, cfg); err == nil {
			in.Followers = fl
		} else {
			return err
		}

		if likes, cm, err := getPostInfo(in.UserId, cfg); err == nil {
			in.AvgLikes = likes
			in.AvgComments = cm
		} else {
			return err
		}
	}
	return nil
}
