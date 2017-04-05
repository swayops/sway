package influencer

import (
	"log"
	"math"

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

	// Have all attrs been set already?
	Attributed bool `json:"scAttr,omitempty"`
	// How many times have we tried getting data on this user?
	Attempts int32 `json:"attempts,omitempty"`

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

func (sc *Scrap) GetMatchingCampaign(cmps map[string]common.Campaign, budgetDb *bolt.DB, cfg *config.Config) *common.Campaign {
	// Get all campaigns that match the platform setting for the campaign
	var considered []*common.Campaign
	for _, cmp := range cmps {
		if sc.Match(cmp, budgetDb, cfg, false, 0) {
			considered = append(considered, &cmp)
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

func (sc *Scrap) Match(cmp common.Campaign, budgetDb *bolt.DB, cfg *config.Config, forecast bool, spendable float64) bool {
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
	}

	// Optimization
	if spendable == 0 {
		store, _ := budget.GetBudgetInfo(budgetDb, cfg, cmp.Id, "")
		if store.IsClosed(&cmp) {
			return false
		}
		spendable = store.Spendable
	}

	if len(cmp.Whitelist) == 0 && !cfg.Sandbox {
		min, max := cmp.GetTargetYield(spendable)
		maxYield := GetMaxYield(&cmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)
		if maxYield < min || maxYield > max || maxYield == 0 {
			return false
		}
	}

	if len(cmp.Whitelist) > 0 {
		return false
	}

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

	if len(cmp.Keywords) > 0 {
		kwFound := false
	L1:
		for _, scKw := range sc.Keywords {
			for _, kw := range cmp.Keywords {
				if kw == scKw {
					kwFound = true
					break L1
				}
			}
		}
		if !kwFound {
			return false
		}
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

	// Category Checks
	if len(cmp.Categories) > 0 {
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

		if !catFound {
			return false
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

		email := templates.ScrapFirstEmail.Render(map[string]interface{}{"Name": sc.Name, "email": sc.EmailAddress})
		if resp, err := cfg.ReplyMailClient().SendMessage(email, "Hey", sc.EmailAddress, sc.Name,
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

func getBiggestBudget(considered []*common.Campaign) *common.Campaign {
	if len(considered) == 0 {
		return nil
	}

	var highest *common.Campaign
	for _, cmp := range considered {
		if highest == nil || cmp.Budget > highest.Budget {
			highest = cmp
		}
	}

	return highest
}

func roundUp(input float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal = round / pow
	return
}
