package influencer

import (
	"log"
	"math"
	"strings"

	"github.com/boltdb/bolt"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

type Scrap struct {
	FullName string `json:"fullName,omitempty"` // Full name

	Name         string `json:"name,omitempty"` // Social media handle
	EmailAddress string `json:"email,omitempty"`
	Followers    int64  `json:"followers,omitempty"`

	Geo *geo.GeoRecord `json:"geo,omitempty"`

	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	Twitter   bool `json:"twitter,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`

	Male   bool `json:"male,omitempty"`
	Female bool `json:"female,omitempty"`

	Categories []string `json:"categories,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`

	// Last time attrs were set
	Updated int32 `json:"updated,omitempty"`
	// How many times have we tried (and failed) getting data on this user?
	Fails int32 `json:"fails,omitempty"`

	// Set internally
	Id         string  `json:"id,omitempty"`
	SentEmails []int32 `json:"sent,omitempty"` // Email TS

	// Set when the scrap has unsubscribed
	Ignore bool `json:"ignore,omitempty"`

	FBData    *facebook.Facebook   `json:"fbData,omitempty"`
	InstaData *instagram.Instagram `json:"instaData,omitempty"`
	TWData    *twitter.Twitter     `json:"twData,omitempty"`
	YTData    *youtube.YouTube     `json:"ytData,omitempty"`
}

func (sc *Scrap) GetMatchingCampaign(campaigns map[string]common.Campaign, audiences *common.Audiences, db *bolt.DB, cfg *config.Config) common.Campaign {
	// Get all campaigns that match the platform setting for the campaign
	var considered []common.Campaign
	for _, cmp := range campaigns {
		if sc.Match(cmp, audiences, db, cfg, nil, false) {
			considered = append(considered, cmp)
		}
	}

	return getBiggestBudget(considered)
}

func (sc *Scrap) GetProfilePicture() string {
	if sc.FBData != nil && sc.FBData.ProfilePicture != "" {
		return sc.FBData.ProfilePicture
	}
	if sc.InstaData != nil && sc.InstaData.ProfilePicture != "" {
		return sc.InstaData.ProfilePicture
	}
	if sc.TWData != nil && sc.TWData.ProfilePicture != "" {
		return sc.TWData.ProfilePicture
	}
	if sc.YTData != nil && sc.YTData.ProfilePicture != "" {
		return sc.YTData.ProfilePicture
	}

	return ""
}

func (sc *Scrap) IsSearchInUsername(p string) bool {
	p = strings.ToLower(p)
	if sc.FBData != nil && strings.Contains(strings.ToLower(sc.FBData.Id), p) {
		return true
	}

	if sc.InstaData != nil && strings.Contains(strings.ToLower(sc.InstaData.UserName), p) {
		return true
	}

	if sc.TWData != nil && strings.Contains(strings.ToLower(sc.TWData.Id), p) {
		return true
	}

	if sc.YTData != nil && strings.Contains(strings.ToLower(sc.YTData.UserName), p) {
		return true
	}

	return false
}

func (sc *Scrap) GetAvgEngs() int64 {
	var engs int64
	if sc.FBData != nil {
		engs += int64(sc.FBData.AvgComments + sc.FBData.AvgLikes + sc.FBData.AvgShares)
	}

	if sc.InstaData != nil {
		engs += int64(sc.InstaData.AvgComments + sc.InstaData.AvgLikes)
	}

	if sc.TWData != nil {
		engs += int64(sc.TWData.AvgLikes + sc.TWData.AvgRetweets)
	}

	if sc.YTData != nil {
		engs += int64(sc.YTData.AvgComments + sc.YTData.AvgViews + sc.YTData.AvgLikes + sc.YTData.AvgDislikes)
	}

	return engs
}

func (sc *Scrap) GetFollowers() int64 {
	var flws int64
	if sc.FBData != nil {
		flws += int64(sc.FBData.Followers)
	}

	if sc.InstaData != nil {
		flws += int64(sc.InstaData.Followers)
	}

	if sc.TWData != nil {
		flws += int64(sc.TWData.Followers)
	}

	if sc.YTData != nil {
		flws += int64(sc.YTData.Subscribers)
	}

	return flws
}

func (sc *Scrap) GetDescription() string {
	if sc.InstaData != nil && sc.InstaData.Bio != "" {
		return sc.InstaData.Bio
	}

	return ""
}

func (sc *Scrap) IsProfilePictureActive() bool {
	if sc.FBData != nil && sc.FBData.ProfilePicture != "" {
		if misc.Ping(sc.FBData.ProfilePicture) != nil {
			return false
		}
	}
	if sc.InstaData != nil && sc.InstaData.ProfilePicture != "" {
		if misc.Ping(sc.InstaData.ProfilePicture) != nil {
			return false
		}
	}
	if sc.TWData != nil && sc.TWData.ProfilePicture != "" {
		if misc.Ping(sc.TWData.ProfilePicture) != nil {
			return false
		}
	}
	if sc.YTData != nil && sc.YTData.ProfilePicture != "" {
		if misc.Ping(sc.YTData.ProfilePicture) != nil {
			return false
		}
	}

	return true
}

func (sc *Scrap) Match(cmp common.Campaign, audiences *common.Audiences, db *bolt.DB, cfg *config.Config, store *budget.Store, forecast bool) bool {
	maxYield := GetMaxYield(&cmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)

	// Social Media Checks
	socialMediaFound := false
	if cmp.YouTube && sc.YouTube {
		socialMediaFound = true
	}

	if cmp.Instagram && sc.Instagram {
		socialMediaFound = true
	}

	if cmp.Twitter && sc.Twitter {
		socialMediaFound = true
	}

	if cmp.Facebook && sc.Facebook {
		socialMediaFound = true
	}

	if !socialMediaFound {
		return false
	}

	if !geo.IsGeoMatch(cmp.Geos, sc.Geo) {
		return false
	}

	// Gender check
	if !cmp.Male && cmp.Female && !sc.Female {
		// Only want females
		return false
	} else if cmp.Male && !cmp.Female && !sc.Male {
		// Only want males
		return false
	} else if !cmp.Male && !cmp.Female {
		return false
	}

	// Follower check
	if cmp.FollowerTarget != nil && !cmp.FollowerTarget.InRange(sc.GetFollowers()) {
		return false
	}

	// Engagements check
	if cmp.EngTarget != nil && !cmp.EngTarget.InRange(sc.GetAvgEngs()) {
		return false
	}

	// Price check
	if cmp.PriceTarget != nil && !cmp.PriceTarget.InRange(maxYield) {
		return false
	}

	// Category Checks
	if len(cmp.Categories) > 0 || len(cmp.Audiences) > 0 || len(cmp.Keywords) > 0 {
		catFound := false
	L2:
		for _, cat := range cmp.Categories {
			for _, scCat := range sc.Categories {
				if cat == scCat {
					catFound = true
					break L2
				}
			}
		}

		// Audience check!
		if !catFound {
			for _, targetAudience := range cmp.Audiences {
				if audiences.IsAllowed(targetAudience, sc.EmailAddress) {
					// There was an audience target and they're  in it!
					catFound = true
					break
				}
			}
		}

		if !catFound {
			for _, kw := range cmp.Keywords {
				if catFound {
					break
				}

				for _, scKw := range sc.Keywords {
					if common.IsExactMatch(kw, scKw) {
						catFound = true
						break
					}
				}

				if sc.InstaData != nil && sc.InstaData.Bio != "" {
					if common.IsExactMatch(sc.InstaData.Bio, kw) {
						catFound = true
						break
					}
				}

				if sc.IsSearchInUsername(kw) {
					catFound = true
					break
				}
			}
		}

		if !catFound {
			return false
		}
	}

	if !forecast {
		// Check if there's an available deal
		var dealFound bool
		for _, deal := range cmp.Deals {
			if deal.IsAvailable() {
				dealFound = true
				break
			}
		}

		if !dealFound {
			return false
		}

		// Check if scrap satisfies the plan
		if !subscriptions.CanInfluencerRun(cmp.AgencyId, cmp.Plan, sc.Followers) {
			return false
		}

		// If it's a perk campaign make sure there are perks available
		if cmp.Perks != nil && cmp.Perks.Count == 0 {
			return false
		}

		// Whitelist check!
		if len(cmp.Whitelist) > 0 {
			_, ok := cmp.Whitelist[sc.EmailAddress]
			if !ok {
				// There was a whitelist and they're not in it!
				return false
			}
		}

		// Optimization
		if !cmp.IsProductBasedBudget() && len(cmp.Whitelist) == 0 && !cfg.Sandbox && cmp.Perks != nil && cmp.Perks.GetType() == "Product" {
			var budgetStore *budget.Store
			if forecast {
				budgetStore = store
			} else {
				budgetStore, _ = budget.GetCampaignStoreFromDb(db, cfg, cmp.Id, cmp.AdvertiserId)
			}

			if budgetStore.IsClosed(&cmp) {
				return false
			}

			min, max := cmp.GetTargetYield(budgetStore.Spendable)
			if maxYield < min || maxYield > max || maxYield == 0 {
				return false
			}
		}
	}

	return true
}

func (sc *Scrap) Email(cmp *common.Campaign, spendable float64, cfg *config.Config) bool {
	if cfg.ReplyMailClient() == nil || sc.Ignore {
		return false
	}

	// Truncate
	spendable = roundUp(spendable, 2)

	perks := "N/A"
	if cmp.Perks != nil {
		perks = cmp.Perks.Name
	}

	// Emailing based on number of times a scrap has been
	// emailed
	if len(sc.SentEmails) == 0 {
		if cfg.Sandbox {
			return true
		}

		email := templates.ScrapFirstEmail.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name, "email": sc.EmailAddress, "Payout": spendable, "Perks": perks, "Task": cmp.Task})
		if resp, err := cfg.ReplyMailClient().SendMessage(email, "A few brands currently requesting you", "shahzil@swayops.com", sc.Name,
			[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			log.Println("Error emailing scrap!", err)
			return false
		}

		if err := cfg.Loggers.Log("email", map[string]interface{}{
			"tag":  "first scrap email",
			"id":   sc.Id,
			"cids": []string{cmp.Id},
		}); err != nil {
			log.Println("Failed to log scrap email!", sc.Id, cmp.Id)
		}

		return true
	} else if len(sc.SentEmails) == 1 {
		// Send second email if it's been more than 48 hours
		if !misc.WithinLast(sc.SentEmails[0], 48) {
			if cfg.Sandbox {
				return true
			}

			email := templates.ScrapDealOne.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name, "email": sc.EmailAddress, "Payout": spendable, "Perks": perks, "Task": cmp.Task})
			if resp, err := cfg.ReplyMailClient().SendMessage(email, "A few brands currently requesting you", sc.EmailAddress, sc.Name,
				[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
				log.Println("Error emailing scrap!", err)
				return false
			}

			if err := cfg.Loggers.Log("email", map[string]interface{}{
				"tag":  "second scrap email",
				"id":   sc.Id,
				"cids": []string{cmp.Id},
			}); err != nil {
				log.Println("Failed to log second scrap email!", sc.Id, cmp.Id)
			}

			return true
		}
	} else if len(sc.SentEmails) == 2 {
		// Send third email if it's been more than 7 days
		if !misc.WithinLast(sc.SentEmails[1], 24*7) {
			if cfg.Sandbox {
				return true
			}

			email := templates.ScrapDealTwo.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name, "email": sc.EmailAddress, "Payout": spendable, "Perks": perks, "Task": cmp.Task})
			if resp, err := cfg.ReplyMailClient().SendMessage(email, "Influencer booking", sc.EmailAddress, sc.Name,
				[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
				log.Println("Error emailing scrap!", err)
				return false
			}

			if err := cfg.Loggers.Log("email", map[string]interface{}{
				"tag":  "third scrap email",
				"id":   sc.Id,
				"cids": []string{cmp.Id},
			}); err != nil {
				log.Println("Failed to log third scrap email!", sc.Id, cmp.Id)
			}

			return true
		}
	}

	return false
}

func getBiggestBudget(considered []common.Campaign) (highest common.Campaign) {
	if len(considered) == 0 {
		return
	}

	for _, cmp := range considered {
		if highest.Id == "" || cmp.Budget > highest.Budget {
			highest = cmp
		}
	}

	return
}

func roundUp(input float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal = round / pow
	return
}
