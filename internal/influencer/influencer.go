package influencer

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/budget"
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
	Name string `json:"name,omitempty"`

	InstagramId string `json:"instagram,omitempty"`
	FbId        string `json:"facebook,omitempty"`
	TwitterId   string `json:"twitter,omitempty"`
	YouTubeId   string `json:"youtube,omitempty"`

	AgencyId   string          `json:"agencyId,omitempty"` // Agency this influencer belongs to
	Geo        *misc.GeoRecord `json:"geo,omitempty"`      // User inputted geo via app
	Gender     string          `json:"gender,omitempty"`
	Categories []string        `json:"categories,omitempty"`
}

type Influencer struct {
	Id string `json:"id"`

	// Name of the influencer
	Name string `json:"name"`

	// Agency this influencer belongs to
	AgencyId string `json:"agencyId,omitempty"`

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
	Categories []string `json:"categories,omitempty"`

	// Active accepted deals by the influencer that have not yet been completed
	ActiveDeals []*common.Deal `json:"activeDeals,omitempty"`
	// Completed and approved deals by the influencer
	CompletedDeals []*common.Deal `json:"completedDeals,omitempty"`

	// Number of times the influencer has unassigned themself from a deal
	Cancellations int32 `json:"cancellations,omitempty"`
	// Number of times the influencer has timed out on a deal
	Timeouts int32 `json:"timeouts,omitempty"`

	// Sway Rep scores by date
	Rep        map[string]float32 `json:"historicRep,omitempty"`
	CurrentRep float32            `json:"currentRep,omitempty"`
}

func New(name, twitterId, instaId, fbId, ytId, gender, agency string, cats []string, geo *misc.GeoRecord, cfg *config.Config) (*Influencer, error) {
	inf := &Influencer{
		Name:       name,
		Id:         misc.PseudoUUID(),
		AgencyId:   agency,
		Geo:        geo,
		Gender:     gender,
		Categories: cats,
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

	inf.setSwayRep()

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

	inf.setSwayRep()

	return nil
}

func (inf *Influencer) UpdateCompletedDeals(cfg *config.Config, activeCampaigns map[string]*common.Campaign) (err error) {
	// Update data for all completed deal posts
	for _, deal := range inf.CompletedDeals {
		if _, ok := activeCampaigns[deal.CampaignId]; !ok {
			// Don't udpate deals for campaigns that aren't
			// active anymore! NO POINT!
			continue
		}
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

func (inf *Influencer) GetPlatformId(deal *common.Deal) string {
	// Gets the user id for the platform based on the deal
	if deal.Tweet != nil && inf.Twitter != nil {
		return inf.Twitter.Id
	} else if deal.Facebook != nil && inf.Facebook != nil {
		return inf.Facebook.Id
	} else if deal.Instagram != nil && inf.Instagram != nil {
		return inf.Instagram.UserName
	} else if deal.YouTube != nil && inf.YouTube != nil {
		return inf.YouTube.UserName
	}
	return ""
}

func (inf *Influencer) setSwayRep() {
	// Considers the following and returns a sway rep score:
	// - Averages per post (likes, comments, shares etc)
	// - Followers
	// - Completed deals
	// - Timeouts
	// - Cancellations

	var rep float32
	if inf.Facebook != nil {
		rep += inf.Facebook.GetScore()
	}
	if inf.Instagram != nil {
		rep += inf.Instagram.GetScore()
	}
	if inf.Twitter != nil {
		rep += inf.Twitter.GetScore()
	}
	if inf.YouTube != nil {
		rep += inf.YouTube.GetScore()
	}

	rep = rep * (1 + float32(len(inf.CompletedDeals))*float32(0.5))
	rep = rep / (1 + float32(inf.Timeouts)*float32(0.5))
	rep = rep / (1 + float32(inf.Cancellations)*1)

	if inf.Rep == nil {
		inf.Rep = make(map[string]float32)
	}

	inf.Rep[getDate()] = rep
	inf.CurrentRep = rep
}

func GetAvailableDeals(campaigns *common.Campaigns, db, budgetDb *bolt.DB, infId, forcedDeal string, geo *misc.GeoRecord, skipGeo bool, cfg *config.Config) []*common.Deal {
	// Iterates over all available deals in the system and matches them
	// with the given influencer
	var (
		v   []byte
		err error
		inf Influencer
	)

	db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.Bucket.Influencer)).Get([]byte(infId))
		return nil
	})

	var infDeals []*common.Deal
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

	for _, cmp := range campaigns.GetStore() {
		targetDeal := &common.Deal{}
		dealFound := false

		if !cmp.IsValid() {
			continue
		}

		for _, deal := range cmp.Deals {
			if deal.Assigned == 0 && deal.Completed == 0 && deal.InfluencerId == "" && !dealFound {
				if forcedDeal != "" && deal.Id != forcedDeal {
					continue
				}
				// Make a copy of the deal rather than assign the pointer
				*targetDeal = *deal
				dealFound = true
			}
		}

		if !dealFound {
			// This campaign has no active deals
			continue
		}

		// Filter Checks
		if len(cmp.Categories) > 0 {
			catFound := false
			for _, cat := range cmp.Categories {
				for _, infCat := range inf.Categories {
					if infCat == cat {
						catFound = true
						break
					}
				}
			}
			if !catFound {
				continue
			}
		}

		// If you already have a/have done deal for this campaign, screw off
		dealFound = false
		for _, d := range inf.ActiveDeals {
			if d.CampaignId == targetDeal.CampaignId {
				dealFound = true
			}
		}
		for _, d := range inf.CompletedDeals {
			if d.CampaignId == targetDeal.CampaignId {
				dealFound = true
			}
		}
		if dealFound {
			continue
		}

		// Match Campaign Geo Targeting with Influencer Geo //
		if !misc.IsGeoMatch(cmp.Geos, geo) {
			continue
		}

		// Gender check
		if !strings.Contains(cmp.Gender, inf.Gender) {
			continue
		}

		// Social Media Checks
		// Values are potential price points TBD
		if cmp.Twitter && inf.Twitter != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Twitter)
		}

		if cmp.Facebook && inf.Facebook != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Facebook)
		}

		if cmp.Instagram && inf.Instagram != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.Instagram)
		}

		if cmp.YouTube && inf.YouTube != nil {
			targetDeal.Platforms = append(targetDeal.Platforms, platform.YouTube)

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
	}

	// Fill in available spendables now
	// This also makes sure that only campaigns with spendable are the
	// only ones eligible for deals
	filtered := make([]*common.Deal, 0, len(infDeals))
	for _, deal := range infDeals {
		store, err := budget.GetBudgetInfo(budgetDb, cfg, deal.CampaignId, "")
		if err == nil && store != nil && store.Spendable > 0 && store.Spent < store.Budget {
			deal.Spendable = store.Spendable
			filtered = append(filtered, deal)
		}
	}

	return filtered
}
