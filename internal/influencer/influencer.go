package influencer

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/tumblr"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

var (
	ErrBadGender = errors.New("Please provide a gender ('m' or 'f')")
	ErrNoAgency  = errors.New("Please provide an agency id")
	ErrNoGeo     = errors.New("Please provide a geo")
)

type InfluencerLoad struct {
	InstagramId string `json:"instagram,omitempty"`
	FbId        string `json:"facebook,omitempty"`
	TwitterId   string `json:"twitter,omitempty"`
	YouTubeId   string `json:"youtube,omitempty"`
	TumblrId    string `json:"tumblr,omitempty"`

	GroupIds []string `json:"groupIds,omitempty"` // Groups this influencer belongs to
	AgencyId string   `json:"agencyId,omitempty"` // Agency this influencer belongs to

	FloorPrice float32 `json:"floor,omitempty"` // Price per engagement set by agency

	Geo *misc.GeoRecord `json:"geo,omitempty"` // User inputted geo via app

	Gender string `json:"gender,omitempty"`
}

type Influencer struct {
	Id         string   `json:"id"`
	GroupIds   []string `json:"groupIds,omitempty"` // Each influencer will be put into multiple groups (owned by agencies)
	AgencyId   string   `json:"agencyId,omitempty"` // agency this influencer belongs to
	FloorPrice float32  `json:"floor,omitempty"`    // Price per engagement set by agency

	Facebook  *facebook.Facebook   `json:"facebook,omitempty"`
	Instagram *instagram.Instagram `json:"instagram,omitempty"`
	Twitter   *twitter.Twitter     `json:"twitter,omitempty"`
	YouTube   *youtube.YouTube     `json:"youtube,omitempty"`
	Tumblr    *tumblr.Tumblr       `json:"tumblr,omitempty"`

	Geo *misc.GeoRecord `json:"geo,omitempty"` // User inputted geo via app

	// Gender
	Gender string `json:"gender,omitempty"` // "m" or "f"

	ActiveDeals   []*common.Deal `json:"activeDeals,omitempty"`   // Accepted pending deals to be completed
	HistoricDeals []*common.Deal `json:"historicDeals,omitempty"` // Contains historic deals completed

	Cancellations int32 `json:"cancel,omitempty"` // How many times has this influencer cancelled a deal? Should affect sway score
}

func New(twitterId, instaId, fbId, ytId, tumblrId, gender, agency string, groupIds []string, floorPrice float32, geo *misc.GeoRecord, cfg *config.Config) (*Influencer, error) {

	if gender != "m" && gender != "f" {
		return nil, ErrBadGender
	}

	if agency == "" {
		return nil, ErrNoAgency
	}

	if geo == nil {
		return nil, ErrNoGeo
	}

	inf := &Influencer{
		Id:         misc.PseudoUUID(), // Possible change to standard numbering?
		AgencyId:   agency,
		GroupIds:   groupIds,
		FloorPrice: floorPrice,
		Geo:        geo,
		Gender:     gender,
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

	if err = inf.NewTumblr(tumblrId, cfg); err != nil {
		return inf, err
	}

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

func (inf *Influencer) NewTumblr(id string, cfg *config.Config) error {
	if len(id) > 0 {
		tr, err := tumblr.New(id, cfg)
		if err != nil {
			return err
		}
		inf.Tumblr = tr
	}
	return nil
}

func GetAvailableDeals(db *bolt.DB, infId, forcedDeal string, geo *misc.GeoRecord, skipGeo bool, cfg *config.Config) []*common.Deal {
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
			if len(cmp.GroupIds) > 0 && !misc.DoesIntersect(cmp.GroupIds, inf.GroupIds) {
				return nil
			}

			// If you already have a deal for this campaign, screw off
			for _, d := range inf.ActiveDeals {
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

			// Insert Age Check here//
			// Insert Follower Check here //

			// Social Media Checks
			if cmp.Twitter && inf.Twitter != nil {
				targetDeal.Platforms[platform.Twitter] = 0
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

			if cmp.Tumblr && inf.Tumblr != nil {
				targetDeal.Platforms[platform.Tumblr] = 4
			}

			// Add deal that has approved platform
			if len(targetDeal.Platforms) > 0 {
				targetDeal.Tag = cmp.Tag
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
