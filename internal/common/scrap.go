package common

import (
	"errors"
	"log"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

var ErrEmail = errors.New("Error sending email!")

type Scrap struct {
	Name         string `json:"name,omitempty"`
	EmailAddress string `json:"email,omitempty"`
	Facebook     bool   `json:"facebook,omitempty"`
	Instagram    bool   `json:"instagram,omitempty"`
	Twitter      bool   `json:"twitter,omitempty"`
	YouTube      bool   `json:"youtube,omitempty"`

	// Set internally
	Id         string  `json:"id,omitempty"`
	SentEmails []int32 `json:"sent,omitempty"` // Email TS
}

func (sc *Scrap) GetMatchingCampaign(cmps map[string]Campaign) *Campaign {
	// Get all campaigns that match the platform setting for the campaign
	for _, cmp := range cmps {
		if matchPlatform(sc, &cmp) {
			return &cmp
		}
	}
	return nil
}

func (sc *Scrap) Email(cmp *Campaign, cfg *config.Config) bool {
	if cfg.ReplyMailClient() == nil {
		return false
	}

	// Emailing based on number of times a scrap has been
	// emailed
	if len(sc.SentEmails) == 0 {
		if !cfg.Sandbox {
			email := templates.ScrapFirstEmail.Render(map[string]interface{}{"Name": sc.Name})
			if resp, err := cfg.ReplyMailClient().SendMessage(email, "Hey", sc.EmailAddress, sc.Name,
				[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
				log.Println("Error emailing scrap!", err)
				return false
			}
		}
		return true
	} else if len(sc.SentEmails) == 1 {
		// Send second email if it's been more than 48 hours
		if !misc.WithinLast(sc.SentEmails[0], 48) {
			if !cfg.Sandbox {
				email := templates.ScrapDealOne.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name})
				if resp, err := cfg.ReplyMailClient().SendMessage(email, "A few brands currently requesting you", sc.EmailAddress, sc.Name,
					[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
					log.Println("Error emailing scrap!", err)
					return false
				}
			}
			return true
		}
	} else if len(sc.SentEmails) == 2 {
		// Send third email if it's been more than 7 days
		if !misc.WithinLast(sc.SentEmails[1], 24*7) {
			if !cfg.Sandbox {
				email := templates.ScrapDealTwo.Render(map[string]interface{}{"Name": sc.Name, "Image": cmp.ImageURL, "Company": cmp.Company, "Campaign": cmp.Name})
				if resp, err := cfg.ReplyMailClient().SendMessage(email, "Influencer booking", sc.EmailAddress, sc.Name,
					[]string{""}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
					log.Println("Error emailing scrap!", err)
					return false
				}
			}
			return true
		}
	}

	return false
}

func matchPlatform(sc *Scrap, cmp *Campaign) bool {
	if cmp.Twitter && sc.Twitter {
		return true
	}

	if cmp.Facebook && sc.Facebook {
		return true
	}

	if cmp.Instagram && sc.Instagram {
		return true
	}

	if cmp.YouTube && sc.YouTube {
		return true
	}

	return false
}
