package common

import (
	"errors"
	"log"
	"math"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

var ErrEmail = errors.New("Error sending email!")

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
	Attributed bool `json:"attr,omitempty"`
	// How many times have we tried getting data on this user?
	Attempts int32 `json:"attempts,omitempty"`

	// Set internally
	Id         string  `json:"id,omitempty"`
	SentEmails []int32 `json:"sent,omitempty"` // Email TS

	// Set when the scrap has unsubscribed
	Ignore bool `json:"ignore,omitempty"`
}

func (sc *Scrap) GetMatchingCampaign(cmps map[string]Campaign) *Campaign {
	// Get all campaigns that match the platform setting for the campaign
	var considered []*Campaign
	for _, cmp := range cmps {
		if sc.Match(cmp, false) {
			considered = append(considered, &cmp)
		}
	}

	return getBiggestBudget(considered)
}

func (sc *Scrap) Match(cmp Campaign, forecast bool) bool {
	if !forecast {
		// Check if there's an available deal
		var dealFound bool
		for _, deal := range cmp.Deals {
			if deal.Assigned == 0 && deal.Completed == 0 && deal.InfluencerId == "" {
				dealFound = true
				break
			}
		}

		if !dealFound {
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

func (sc *Scrap) Email(cmp *Campaign, spendable float64, cfg *config.Config) bool {
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

func getBiggestBudget(considered []*Campaign) *Campaign {
	if len(considered) == 0 {
		return nil
	}

	var highest *Campaign
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
