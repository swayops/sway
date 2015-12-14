package influencer

import (
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/rtb"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

type Influencer struct {
	Id         string
	CategoryId string  // Each influencer will be put into a category
	AgencyId   string  // Groups this influencer belongs to (agencies, brands view invites)
	FloorPrice float32 // Price per engagement set by agency

	Facebook  *facebook.Facebook
	Instagram *instagram.Instagram
	Twitter   *twitter.Twitter
	YouTube   *youtube.YouTube

	Active   []*rtb.Deal // Accepted pending deals to be completed
	Historic []*rtb.Deal // Contains historic deals completed
}

func New(twitterId, instaId, fbId, ytId string, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id: pseudoUUID(), // Possible change to standard numbering?
	}

	err := inf.NewInsta(instaId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewFb(fbId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewTwitter(twitterId, cfg)
	if err != nil {
		return inf, err
	}

	err = inf.NewYouTube(ytId, cfg)
	if err != nil {
		return inf, err
	}

	// Saving to db functionality TBD.. iodb?
	return inf, nil
}

// New functions can be re-used later if an influencer
// adds a new social media account
func (inf *Influencer) NewFb(id string, cfg *config.Config) error {
	if len(id) > 0 {
		fb, err := facebook.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Facebook = fb
	}
	return nil
}

func (inf *Influencer) NewInsta(id string, cfg *config.Config) error {
	if len(id) > 0 {
		insta, err := instagram.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Instagram = insta
	}
	return nil
}

func (inf *Influencer) NewTwitter(id string, cfg *config.Config) error {
	if len(id) > 0 {
		tw, err := twitter.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Twitter = tw
	}
	return nil
}

func (inf *Influencer) NewYouTube(id string, cfg *config.Config) error {
	if len(id) > 0 {
		yt, err := youtube.New(id, cfg)
		if err != nil {
			return err
		}
		inf.YouTube = yt
	}
	return nil
}
