package common

import (
	"encoding/json"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

// This deal represents a possible bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id           string `json:"id"`
	CampaignId   string `json:"campaignId"`
	AdvertiserId string `json:"advertiserId"`

	// Platform determined by GetAvailableDeals with value as potential pricepoint
	// This is also saved/reset in the un/assign handlers
	Platforms []string `json:"platforms,omitempty"`

	// Timestamp for when the deal was picked up by an influencer
	Assigned int32 `json:"assigned,omitempty"`
	// Timestamp for when the deal was completed by an influencer
	Completed int32 `json:"completed,omitempty"`

	// All of the following are when a deal is assigned/unassigned
	// or times out
	InfluencerId     string `json:"influencerId,omitempty"`
	AssignedPlatform string `json:"assignedPlatform,omitempty"`

	// Only set once deal is completed. Contain
	// the information for the post which satisfied the deal
	Tweet     *twitter.Tweet  `json:"tweet,omitempty"`
	Facebook  *facebook.Post  `json:"facebook,omitempty"`
	Instagram *instagram.Post `json:"instagram,omitempty"`
	YouTube   *youtube.Post   `json:"youtube,omitempty"`

	PostUrl string `json:"postUrl,omitempty"`

	// Requirements copied from the campaign to the deal
	// GetAvailableDeals
	Tags    []string `json:"hashtags,omitempty"`
	Mention string   `json:"mention,omitempty"`
	Link    string   `json:"link,omitempty"`
	Task    string   `json:"task,omitempty"`
	Perks   string   `json:"perks,omitempty"` // Perks need to be specced out

	// How much this campaign has left to spend for the month
	// Only filled in GetAvailableDeals for the influencer to see
	// and is saved to show how much the influencer was offered
	// when the deal was assigned
	Spendable float64 `json:"spendable,omitempty"`

	// Keyed on month.. showing payouts calculated by month
	Payment map[string]*Money `json:"infPayout,omitempty"`
}

type Money struct {
	// How much has been paid out to the influencer for this deal?
	Influencer float64 `json:"infPayout,omitempty"`
	// How much has been paid out to the agency for this deal?
	Agency   float64 `json:"agencyPayout,omitempty"`
	AgencyId string  `json:"agencyId,omitempty"`
}

func (d *Deal) Pay(inf, agency float64, agId string) {
	if d.Payment == nil {
		d.Payment = make(map[string]*Money)
	}
	key := getMonthKey(0)
	data, ok := d.Payment[key]
	if !ok {
		data = &Money{}
		d.Payment[key] = data
	}

	data.Influencer += inf
	data.Agency += agency
	data.AgencyId = agId
}

func (d *Deal) GetPayout(offset int) (m *Money) {
	key := getMonthKey(offset)
	if d.Payment == nil {
		return
	}
	m, _ = d.Payment[key]
	return
}

func getMonthKey(offset int) string {
	now := time.Now().UTC()
	if offset > 0 {
		offset = -offset
	}
	return now.AddDate(0, offset, 0).Format("01-2006")
}

func GetAllActiveDeals(db *bolt.DB, cfg *config.Config) ([]*Deal, error) {
	// Retrieves all active deals in the system!
	var err error
	deals := []*Deal{}

	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &Campaign{}
			if err = json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return nil
			}

			for _, deal := range cmp.Deals {
				if deal.Assigned > 0 && deal.Completed == 0 && deal.InfluencerId != "" {
					deals = append(deals, deal)
				}
			}
			return
		})
		return nil
	}); err != nil {
		return deals, err
	}
	return deals, err
}
