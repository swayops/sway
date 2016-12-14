package common

import (
	"errors"
	"log"

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
}

func (sc *Scrap) GetMatchingCampaign(cmps map[string]Campaign) *Campaign {
	// Get all campaigns that match the platform setting for the campaign
	var considered []*Campaign
	for _, cmp := range cmps {
		if sc.Match(cmp) {
			considered = append(considered, &cmp)
		}
	}

	return getBiggestBudget(considered)
}

func (sc *Scrap) Match(cmp Campaign) bool {
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

func (sc *Scrap) Email(cmp *Campaign, cfg *config.Config) bool {
	if cfg.ReplyMailClient() == nil {
		return false
	}

	// Emailing based on number of times a scrap has been
	// emailed
	if len(sc.SentEmails) == 0 {
		if cfg.Sandbox {
			return true
		}

		email := templates.ScrapFirstEmail.Render(map[string]interface{}{"Name": sc.Name})
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

			email := templates.ScrapDealOne.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name})
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

			email := templates.ScrapDealTwo.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name})
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
