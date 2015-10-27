package influencer

import (
	"github.com/swayops/sway/internal/config"
	"github.com/swayops/sway/internal/deal"
	"github.com/swayops/sway/internal/facebook"
	"github.com/swayops/sway/internal/instagram"
	"github.com/swayops/sway/internal/twitter"
)

type Influencer struct {
	Id         string
	CategoryId string // Each influencer will be put into a category
	Facebook   *facebook.Facebook
	Instagram  *instagram.Instagram
	Twitter    *twitter.Twitter
	Active     []*deal.Deal // Accepted pending deals to be completed
	Historic   []*deal.Deal // Contains historic deals completed
}

func New(twitterId, instaId, fbId string, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id: pseudoUUID(), // Possible change to standard numbering?
	}

	err := inf.NewFb(fbId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewInsta(instaId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewTwitter(twitterId, cfg)
	if err != nil {
		return inf, err
	}

	// Saving to db functionality TBD.. iodb?
	return inf, nil
}

func (inf *Influencer) NewFb(id string, cfg *config.Config) error {
	if len(id) > 0 {
		fb, err := facebook.New(id, cfg.FbEndpoint)
		if err != nil {
			return err
		}
		inf.Facebook = fb
	}
	return nil
}

func (inf *Influencer) NewInsta(id string, cfg *config.Config) error {
	if len(id) > 0 {
		insta, err := instagram.New(id, cfg.InstaEndpoint)
		if err != nil {
			return err
		}
		inf.Instagram = insta
	}
	return nil
}

func (inf *Influencer) NewTwitter(id string, cfg *config.Config) error {
	if len(id) > 0 {
		tw, err := twitter.New(id, cfg.TwitterEndpoint)
		if err != nil {
			return err
		}
		inf.Twitter = tw
	}
	return nil
}
