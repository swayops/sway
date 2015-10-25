package influencer

import (
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

func New(twitterId, instaId, fbId string) (*Influencer, error) {
	inf := &Influencer{
		Id: pseudoUUID(), // Possible change to standard numbering?
	}

	if len(twitterId) > 0 {
		tw, err := twitter.New(twitterId, twEndpoint)
		if err != nil {
			return inf, err
		}
		inf.Twitter = tw
	}

	if len(instaId) > 0 {
		insta, err := instagram.New(instaId, instaEndpoint)
		if err != nil {
			return inf, err
		}
		inf.Instagram = insta
	}

	if len(fbId) > 0 {
		fb, err := facebook.New(fbId, fbEndpoint)
		if err != nil {
			return inf, err
		}
		inf.Facebook = fb
	}

	// Saving to db functionality TBD
	return inf, nil
}
