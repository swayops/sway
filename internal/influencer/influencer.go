package influencer

import (
	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/internal/facebook"
	"github.com/swayops/sway/internal/instagram"
	"github.com/swayops/sway/internal/twitter"
)

type Influencer struct {
	Id        string
	Facebook  *facebook.Facebook
	Instagram *instagram.Instagram
	Twitter   *twitter.Twitter
}

const (
	twEndpoint    = "google.com"
	fbEndpoint    = "google.com"
	instaEndpoint = "google.com"
)

func New(twitterId, instaId, fbId string, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id: pseudoUUID(), // Possible change to standard numbering?
	}

	err := inf.UpdateFb(fbId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.UpdateInsta(instaId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.UpdateTwitter(twitterId, cfg)
	if err != nil {
		return inf, err
	}

	// Saving to db functionality TBD
	return inf, nil
}

func (inf *Influencer) UpdateTwitter(id string, cfg *config.Config) err {
	if len(id) > 0 {
		tw, err := twitter.New(id, cfg.TwitterEndpoint)
		if err != nil {
			return err
		}
		inf.Twitter = tw
	}
	return nil
}

func (inf *Influencer) UpdateInsta(id string, cfg *config.Config) err {
	if len(id) > 0 {
		insta, err := instagram.New(id, cfg.InstaEndpoint)
		if err != nil {
			return err
		}
		inf.Instagram = insta
	}
	return nil
}

func (inf *Influencer) UpdateFb(id string, cfg *config.Config) err {
	if len(id) > 0 {
		fb, err := Facebook.New(id, cfg.FbEndpoint)
		if err != nil {
			return err
		}
		inf.Facebook = fb
	}
	return nil
}
