package influencer

import (
	"encoding/json"
	"log"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

// The json struct accepted by the putInfluencer method
type InfluencerLoad struct {
	InstagramId string `json:"instagram,omitempty"`
	FbId        string `json:"facebook,omitempty"`
	TwitterId   string `json:"twitter,omitempty"`
	YouTubeId   string `json:"youtube,omitempty"`

	AgencyId   string          `json:"agencyId,omitempty"` // Agency this influencer belongs to
	FloorPrice float32         `json:"floor,omitempty"`    // Price per engagement set by agency
	Geo        *misc.GeoRecord `json:"geo,omitempty"`      // User inputted geo via app
	Gender     string          `json:"gender,omitempty"`
	Category   string          `json:"category,omitempty"`
}

type Influencer struct {
	Id string `json:"id"`

	// Agency this influencer belongs to
	AgencyId string `json:"agencyId,omitempty"`

	// Minimum price per engagement set by agency
	FloorPrice float32 `json:"floor,omitempty"`

	// References to the social media accounts this influencer owns
	Facebook  *facebook.Facebook   `json:"facebook,omitempty"`
	Instagram *instagram.Instagram `json:"instagram,omitempty"`
	Twitter   *twitter.Twitter     `json:"twitter,omitempty"`
	YouTube   *youtube.YouTube     `json:"youtube,omitempty"`

	// User inputted geo (should be ingested by the app)
	Geo *misc.GeoRecord `json:"geo,omitempty"`

	// "m" or "f" or "unicorn" lol
	Gender string `json:"gender,omitempty"`
	// Influencer inputted category they belong to
	Category string `json:"category,omitempty"`

	// Active accepted deals by the influencer that have not yet been completed
	ActiveDeals []*common.Deal `json:"activeDeals,omitempty"`
	// Completed and approved deals by the influencer
	CompletedDeals []*common.Deal `json:"completedDeals,omitempty"`

	// Number of times the influencer has unassigned themself from a deal
	Cancellations int32 `json:"cancellations,omitempty"`
	// Number of times the influencer has timed out on a deal
	Timeouts int32 `json:"timeouts,omitempty"`
}

func New(twitterId, instaId, fbId, ytId, gender, agency, cat string, floorPrice float32, geo *misc.GeoRecord, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Id:         misc.PseudoUUID(),
		AgencyId:   agency,
		FloorPrice: floorPrice,
		Geo:        geo,
		Gender:     gender,
		Category:   cat,
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

	return inf, nil
}

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

func (inf *Influencer) UpdateAll(cfg *config.Config) (err error) {
	if inf.Facebook != nil {
		if err = inf.Facebook.UpdateData(cfg); err != nil {
			return err
		}
	}
	if inf.Instagram != nil {
		if err = inf.Instagram.UpdateData(cfg); err != nil {
			return err
		}
	}
	if inf.Twitter != nil {
		if err = inf.Twitter.UpdateData(cfg); err != nil {
			return err
		}
	}
	if inf.YouTube != nil {
		if err = inf.YouTube.UpdateData(cfg); err != nil {
			return err
		}
	}
	return nil
}

func (inf *Influencer) UpdateCompletedDeals(cfg *config.Config) (err error) {
	// Update data for all completed deal posts
	for _, deal := range inf.CompletedDeals {
		if deal.Tweet != nil {
			if err := deal.Tweet.UpdateData(cfg); err != nil {
				return err
			}
		} else if deal.Facebook != nil {
			if err := deal.Facebook.UpdateData(cfg); err != nil {
				return err
			}
		} else if deal.Instagram != nil {
			if err := deal.Instagram.UpdateData(cfg); err != nil {
				return err
			}
		} else if deal.YouTube != nil {
			if err := deal.YouTube.UpdateData(cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetAvailableDeals(db *bolt.DB, infId, forcedDeal string, geo *misc.GeoRecord, skipGeo bool, cfg *config.Config) []*common.Deal {
	// Iterates over all available deals in the system and matches them
	// with the given influencer

	var (
		v   []byte
		err error
		inf Influencer
	)
	infDeals := make([]*common.Deal, 0, 512)

	db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.Bucket.Influencer)).Get([]byte(infId))
		return nil
	})

	if err = json.Unmarshal(v, &inf); err != nil {
		log.Println("Error unmarshalling influencer", err)
		return infDeals
	}

	if geo == nil && !skipGeo {
		if inf.Geo != nil {
			geo = inf.Geo
		} else {
			if inf.Instagram != nil && inf.Instagram.LastLocation != nil {
				geo = inf.Instagram.LastLocation
			} else if inf.Twitter != nil && inf.Twitter.LastLocation != nil {
				geo = inf.Twitter.LastLocation
			}
		}

	}

	db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Campaign)).ForEach(func(cid, v []byte) (err error) {
			cmp := &common.Campaign{}
			targetDeal := &common.Deal{}
			dealFound := false

			if err := json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return nil
			}

			if !cmp.Active || cmp.Budget == 0 || len(cmp.Deals) == 0 {
				return nil
			}

			for _, deal := range cmp.Deals {
				if deal.Assigned == 0 && deal.Completed == 0 && deal.InfluencerId == "" && !dealFound {
					if forcedDeal != "" && deal.Id != forcedDeal {
						continue
					}
					targetDeal = deal
					dealFound = true
					targetDeal.Platforms = make(map[string]float32)
				}
			}

			if !dealFound {
				// This campaign has no active deals
				return nil
			}

			// Filter Checks
			if len(cmp.Categories) > 0 {
				catFound := false
				for _, cat := range cmp.Categories {
					if inf.Category == cat {
						catFound = true
						break
					}
				}
				if !catFound {
					return nil
				}
			}

			// If you already have a/have done deal for this campaign, screw off
			for _, d := range inf.ActiveDeals {
				if d.CampaignId == targetDeal.CampaignId {
					return nil
				}
			}
			for _, d := range inf.CompletedDeals {
				if d.CampaignId == targetDeal.CampaignId {
					return nil
				}
			}

			// Match Campaign Geo Targeting with Influencer Geo //
			if !misc.IsGeoMatch(cmp.Geos, geo) {
				return nil
			}

			// Gender check
			if cmp.Gender == "m" {
				if inf.Gender == "f" {
					return nil
				}
			} else if cmp.Gender == "f" {
				if inf.Gender == "m" {
					return nil
				}
			}

			// Social Media Checks
			// Values are potential price points TBD
			if cmp.Twitter && inf.Twitter != nil {
				targetDeal.Platforms[platform.Twitter] = 1
			}

			if cmp.Facebook && inf.Facebook != nil {
				targetDeal.Platforms[platform.Facebook] = 1

			}

			if cmp.Instagram && inf.Instagram != nil {
				targetDeal.Platforms[platform.Instagram] = 2

			}

			if cmp.YouTube && inf.YouTube != nil {
				targetDeal.Platforms[platform.YouTube] = 3

			}

			// Add deal that has approved platform
			if len(targetDeal.Platforms) > 0 {
				targetDeal.Tags = cmp.Tags
				targetDeal.Mention = cmp.Mention
				targetDeal.Link = cmp.Link
				targetDeal.Task = cmp.Task
				targetDeal.Perks = cmp.Perks
				infDeals = append(infDeals, targetDeal)
			}
			return
		})
		return nil
	})
	return infDeals
}
