package common

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

	CampaignName  string `json:"cmpName,omitempty"`
	CampaignImage string `json:"cmpImg,omitempty"`
	Company       string `json:"company,omitempty"`

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
	InfluencerName   string `json:"influencerName,omitempty"`
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
	Tags          []string `json:"hashtags,omitempty"`
	Mention       string   `json:"mention,omitempty"`
	Link          string   `json:"link,omitempty"`
	ShortenedLink string   `json:"shortenedLink,omitempty"`

	Task string `json:"task,omitempty"`
	Perk *Perk  `json:"perk,omitempty"`

	// How much this campaign has left to spend for the month
	// Only filled in GetAvailableDeals for the influencer to see
	// and is saved to show how much the influencer was offered
	// when the deal was assigned
	Spendable float64 `json:"spendable,omitempty"`

	// Keyed on DAY.. showing payouts calculated by DAY
	Payment map[string]*Payout `json:"infPayout,omitempty"`
}

type Payout struct {
	// How much has been paid out to the influencer for this deal?
	Influencer float64 `json:"infPayout,omitempty"`
	// How much has been paid out to the agency for this deal?
	Agency   float64 `json:"agencyPayout,omitempty"`
	AgencyId string  `json:"agencyId,omitempty"`

	DSP      float64 `json:"dsp,omitempty"`
	Exchange float64 `json:"exchange,omitempty"`

	Likes    int32 `json:"likes,omitempty"`
	Dislikes int32 `json:"dislikes,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`
	Views    int32 `json:"views,omitempty"`
	Clicks   int32 `json:"clicks,omitempty"`
}

func (p *Payout) TotalMarkup() float64 {
	return p.DSP + p.Exchange + p.Agency
}

func (d *Deal) Pay(inf, agency, dsp, exchange float64, agId string) {
	if d.Payment == nil {
		d.Payment = make(map[string]*Payout)
	}
	key := getDate()
	data, ok := d.Payment[key]
	if !ok {
		data = &Payout{}
		d.Payment[key] = data
	}

	data.DSP += dsp
	data.Exchange += exchange
	data.Influencer += inf
	data.Agency += agency
	data.AgencyId = agId
	log.Println("AGENCY!", agency, data.Agency, agId)
}

func (d *Deal) Incr(likes, dislikes, comments, shares, views int32) {
	if d.Payment == nil {
		d.Payment = make(map[string]*Payout)
	}
	key := getDate()
	data, ok := d.Payment[key]
	if !ok {
		data = &Payout{}
		d.Payment[key] = data
	}

	data.Likes += likes
	data.Dislikes += dislikes
	data.Comments += comments
	data.Shares += shares
	data.Views += views
}

func (d *Deal) Click() {
	if d.Payment == nil {
		d.Payment = make(map[string]*Payout)
	}
	key := getDate()
	data, ok := d.Payment[key]
	if !ok {
		data = &Payout{}
		d.Payment[key] = data
	}

	data.Clicks += 1
}

func (d *Deal) GetPayout(offset int) (m *Payout) {
	key := getMonthOffset(offset)
	if d.Payment == nil {
		return
	}

	data := &Payout{}

	for d, payout := range d.Payment {
		if strings.Index(d, key) == 0 {
			data.DSP += payout.DSP
			data.Exchange += payout.Exchange
			data.Influencer += payout.Influencer
			data.Agency += payout.Agency
			data.AgencyId = payout.AgencyId
		}
	}
	return data
}

func (d *Deal) Get(dates []string) (m *Payout) {
	data := &Payout{}
	for _, date := range dates {
		payout, ok := d.Payment[date]
		if !ok {
			continue
		}

		data.DSP += payout.DSP
		data.Exchange += payout.Exchange
		data.Influencer += payout.Influencer
		data.Agency += payout.Agency
		data.AgencyId = payout.AgencyId

		data.Likes += payout.Likes
		data.Dislikes += payout.Dislikes
		data.Comments += payout.Comments
		data.Shares += payout.Shares
		data.Views += payout.Views
		data.Clicks += payout.Clicks
	}
	return data
}

func (d *Deal) GetByAgency(dates []string, agid string) (m *Payout) {
	data := &Payout{}
	for _, date := range dates {
		payout, ok := d.Payment[date]
		if !ok {
			continue
		}

		if agid != "" && payout.AgencyId != agid {
			continue
		}

		data.DSP += payout.DSP
		data.Exchange += payout.Exchange
		data.Influencer += payout.Influencer
		data.Agency += payout.Agency
		data.AgencyId = payout.AgencyId

		data.Likes += payout.Likes
		data.Dislikes += payout.Dislikes
		data.Comments += payout.Comments
		data.Shares += payout.Shares
		data.Views += payout.Views
		data.Clicks += payout.Clicks
	}
	return data
}

func (d *Deal) Published() int32 {
	if d.Tweet != nil {
		return int32(d.Tweet.CreatedAt.Unix())
	}

	if d.Facebook != nil {
		return int32(d.Facebook.Published.Unix())
	}

	if d.Instagram != nil {
		return d.Instagram.Published
	}

	if d.YouTube != nil {
		return d.YouTube.Published
	}

	return 0
}

const (
	dateFormat  = "%d-%02d-%02d"
	monthFormat = "%d-%02d"
)

func getMonthOffset(offset int) string {
	t := time.Now().UTC()
	t = t.AddDate(0, -offset, 0)
	return fmt.Sprintf(
		monthFormat,
		t.Year(),
		t.Month(),
	)
}

func getDate() string {
	return getDateFromTime(time.Now().UTC())
}

func getDateFromTime(t time.Time) string {
	return fmt.Sprintf(
		dateFormat,
		t.Year(),
		t.Month(),
		t.Day(),
	)
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
