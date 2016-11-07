package common

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

const (
	engagementViewRatio = 0.04
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
	Tags          []string `json:"tags,omitempty"`
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

	// Keyed on DAY.. showing stats calculated by DAY
	Reporting map[string]*Stats `json:"stats,omitempty"`
}

type Stats struct {
	// How much has been paid out to the influencer for this deal?
	Influencer float64 `json:"infStats,omitempty"`
	// How much has been paid out to the agency for this deal?
	Agency   float64 `json:"agencyStats,omitempty"`
	AgencyId string  `json:"agencyId,omitempty"`

	// DSP and Exchange Fees respectively
	DSP      float64 `json:"dsp,omitempty"`
	Exchange float64 `json:"exchange,omitempty"`

	Likes    int32 `json:"likes,omitempty"`
	Dislikes int32 `json:"dislikes,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`
	Views    int32 `json:"views,omitempty"`
	Clicks   int32 `json:"clicks,omitempty"`
	Perks    int32 `json:"perks,omitempty"`
}

func (st *Stats) TotalMarkup() float64 {
	return st.DSP + st.Exchange + st.Agency
}

func (d *Deal) TotalStats() *Stats {
	total := &Stats{}
	for _, data := range d.Reporting {
		total.Likes += data.Likes
		total.Dislikes += data.Dislikes
		total.Comments += data.Comments
		total.Shares += data.Shares
		total.Views += data.Views
		total.Clicks += data.Clicks
		total.Influencer += data.Influencer
		total.Agency += data.Agency
	}
	return total
}

func (d *Deal) Pay(inf, agency, dsp, exchange float64, agId string) {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	key := GetDate()
	data, ok := d.Reporting[key]
	if !ok {
		data = &Stats{}
		d.Reporting[key] = data
	}

	data.DSP += dsp
	data.Exchange += exchange
	data.Influencer += inf
	data.Agency += agency
	data.AgencyId = agId
}

func (d *Deal) Incr(likes, dislikes, comments, shares, views int32) {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	key := GetDate()
	data, ok := d.Reporting[key]
	if !ok {
		data = &Stats{}
		d.Reporting[key] = data
	}
	data.Likes += likes
	data.Dislikes += dislikes
	data.Comments += comments
	data.Shares += shares
	if views > 0 {
		data.Views += views
	} else {
		// Estimate views if there are none
		data.Views += int32(float64(likes+comments+shares) / engagementViewRatio)
	}
}

func (d *Deal) PerkIncr() {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	key := GetDate()
	data, ok := d.Reporting[key]
	if !ok {
		data = &Stats{}
		d.Reporting[key] = data
	}

	data.Perks += 1
}

func (d *Deal) ClearDeltas() {
	// Clears out deltas which are used
	// to deplete budgets!

	if d.Tweet != nil {
		d.Tweet.Clear()
	}

	if d.Facebook != nil {
		d.Facebook.Clear()
	}

	if d.Instagram != nil {
		d.Instagram.Clear()
	}

	if d.YouTube != nil {
		d.YouTube.Clear()
	}
}

func (d *Deal) Click() {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	key := GetDate()
	data, ok := d.Reporting[key]
	if !ok {
		data = &Stats{}
		d.Reporting[key] = data
	}

	data.Clicks += 1
}

func (d *Deal) GetMonthStats(offset int) (m *Stats) {
	// Only returns monetary information
	// Used for billing

	key := GetMonthOffset(offset)
	if d.Reporting == nil {
		return
	}

	data := &Stats{}
	for d, stats := range d.Reporting {
		if strings.Index(d, key) == 0 {
			data.DSP += stats.DSP
			data.Exchange += stats.Exchange
			data.Influencer += stats.Influencer
			data.Agency += stats.Agency
			data.AgencyId = stats.AgencyId
		}
	}
	return data
}

func (d *Deal) Get(dates []string, agid string) (m *Stats) {
	data := &Stats{}
	for _, date := range dates {
		stats, ok := d.Reporting[date]
		if !ok {
			continue
		}

		if agid != "" && stats.AgencyId != agid {
			continue
		}

		data.DSP += stats.DSP
		data.Exchange += stats.Exchange
		data.Influencer += stats.Influencer
		data.Agency += stats.Agency
		data.AgencyId = stats.AgencyId

		data.Likes += stats.Likes
		data.Dislikes += stats.Dislikes
		data.Comments += stats.Comments
		data.Shares += stats.Shares
		data.Views += stats.Views
		data.Clicks += stats.Clicks
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
