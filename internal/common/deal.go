package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"

	"github.com/swayops/converter/pixel"
)

const (
	engagementViewRatio = 0.04
)

var (
	InvalidPostURL = errors.New("Invalid post URL!")
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

	// Stores whether the influencer has been emailed about
	// their deal being picked up
	PickedUp bool `json:"pickedUp,omitempty"`
	// Determines whether there will be fraud checking
	SkipFraud bool `json:"skipFraud,omitempty"`
	// Timestamp for when the deal was picked up by an influencer
	Assigned int32 `json:"assigned,omitempty"`
	// Timestamp for when the deal was completed by an influencer
	Completed int32 `json:"completed,omitempty"`

	// All of the following are when a deal is assigned/unassigned
	// or times out
	InfluencerId   string `json:"influencerId,omitempty"`
	InfluencerName string `json:"influencerName,omitempty"`

	// Assigned when deal is completed
	AssignedPlatform string `json:"assignedPlatform,omitempty"`

	// Only set once deal is completed. Contain
	// the information for the post which satisfied the deal
	Tweet     *twitter.Tweet  `json:"tweet,omitempty"`
	Facebook  *facebook.Post  `json:"facebook,omitempty"`
	Instagram *instagram.Post `json:"instagram,omitempty"`
	YouTube   *youtube.Post   `json:"youtube,omitempty"`

	Bonus *Bonus `json:"bonus,omitempty"`

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
	// Field set by GetAvailableDeals specifying how much the influencer
	// COULD earn on this deal
	LikelyEarnings float64 `json:"likelyEarnings,omitempty"`

	// Keyed on DAY.. showing stats calculated by DAY
	Reporting map[string]*Stats `json:"stats,omitempty"`

	// Used by inf app to decide on whether or not it should show post submission option
	RequiresSubmission bool        `json:"reqSub,omitempty"`
	Submission         *Submission `json:"submission,omitempty"`

	From int64 `json:"fromTime,omitempty"`
	To   int64 `json:"toTime,omitempty"`

	TermsAndConditions string `json:"terms"`
}

type Submission struct {
	ImageData []string `json:"imgData,omitempty"`
	// Could be an array of image URLs, or video URLs
	ContentURL []string `json:"content,omitempty"`

	Message string `json:"caption,omitempty"`

	Approved bool `json:"approved,omitempty"`
}

func (s *Submission) SanitizeContent() {
	// Convert YT and Vimeo URLs into embed-capable URLs
	for idx, content := range s.ContentURL {
		u, err := url.Parse(content)
		if err != nil {
			continue
		}

		if strings.Contains(u.Host, "youtube") {
			qp := u.Query()
			val := qp["v"]
			if len(val) == 0 || val[0] == "" {
				continue
			}

			s.ContentURL[idx] = fmt.Sprintf("https://www.youtube.com/embed/%s", val[0])
		} else if strings.Contains(u.Host, "vimeo") {
			videoID := strings.Trim(u.Path, "/")
			if len(videoID) == 0 {
				continue
			}

			s.ContentURL[idx] = fmt.Sprintf("https://player.vimeo.com/video/%s", videoID)
		}

	}
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
	Perks    int32 `json:"perks,omitempty"`

	Conversions []pixel.Conversion `json:"conversions,omitempty"`

	LegacyClicks int32 `json:"clicks,omitempty"`

	PendingClicks  []*Click `json:"pendingClicks,omitempty"`
	ApprovedClicks []*Click `json:"approvedClicks,omitempty"`
}

type Click struct {
	UUID string `json:"uuid,omitempty"`
	TS   int32  `json:"ts,omitempty"`
}

func (st *Stats) TotalMarkup() float64 {
	return st.DSP + st.Exchange + st.Agency
}

func (st *Stats) GetClicks() int32 {
	return int32(len(st.ApprovedClicks)) + st.LegacyClicks
}

func (st *Stats) GetUniqueClicks() int32 {
	uuids := map[string]bool{}
	for _, cl := range st.ApprovedClicks {
		uuids[cl.UUID] = true
	}

	for _, cl := range st.PendingClicks {
		uuids[cl.UUID] = true
	}

	return int32(len(uuids))
}

func (st *Stats) GetApprovedClickUUIDs() map[string]bool {
	out := map[string]bool{}
	for _, cl := range st.ApprovedClicks {
		out[cl.UUID] = true
	}
	return out
}

type Bonus struct {
	Tweet     []*twitter.Tweet  `json:"tweet,omitempty"`
	Facebook  []*facebook.Post  `json:"facebook,omitempty"`
	Instagram []*instagram.Post `json:"instagram,omitempty"`
	YouTube   []*youtube.Post   `json:"youtube,omitempty"`
}

func (d *Deal) AddBonus(tweet *twitter.Tweet, fbPost *facebook.Post, instaPost *instagram.Post, ytPost *youtube.Post) {
	if d.Bonus == nil {
		d.Bonus = &Bonus{}
	}

	if tweet != nil {
		// Lets make sure this post doesn't already exist!
		for _, d := range d.Bonus.Tweet {
			if d.Id == tweet.Id {
				return
			}
		}

		if d.Tweet != nil && d.Tweet.Id == tweet.Id {
			return
		}

		d.Bonus.Tweet = append(d.Bonus.Tweet, tweet)
	}

	if fbPost != nil {
		// Lets make sure this post doesn't already exist!
		for _, d := range d.Bonus.Facebook {
			if d.Id == fbPost.Id {
				return
			}
		}

		if d.Facebook != nil && d.Facebook.Id == fbPost.Id {
			return
		}

		d.Bonus.Facebook = append(d.Bonus.Facebook, fbPost)
	}

	if instaPost != nil {
		// Lets make sure this post doesn't already exist!
		for _, d := range d.Bonus.Instagram {
			if d.Id == instaPost.Id {
				return
			}
		}

		if d.Instagram != nil && d.Instagram.Id == instaPost.Id {
			return
		}

		d.Bonus.Instagram = append(d.Bonus.Instagram, instaPost)
	}

	if ytPost != nil {
		// Lets make sure this post doesn't already exist!
		for _, d := range d.Bonus.YouTube {
			if d.Id == ytPost.Id {
				return
			}
		}

		if d.YouTube != nil && d.YouTube.Id == ytPost.Id {
			return
		}

		d.Bonus.YouTube = append(d.Bonus.YouTube, ytPost)
	}
}

func (d *Deal) SanitizeClicks(completion int32) map[string]*Stats {
	// Takes in a completion time and calculates which ones were AFTER the completion time!
	// Deletes all pending clicks that were before completion
	reporting := make(map[string]*Stats)

	for key, data := range d.Reporting {
		var filtered []*Click
		for _, click := range data.PendingClicks {
			if click.TS >= completion {
				filtered = append(filtered, click)
			}
		}
		data.PendingClicks = filtered
		reporting[key] = data
	}

	return reporting
}

func (d *Deal) ApproveAllClicks() {
	// Moves all pending clicks to approved!
	for _, data := range d.Reporting {
		data.ApprovedClicks = append(data.ApprovedClicks, data.PendingClicks...)
		data.PendingClicks = nil
	}

	return
}

func (d *Deal) TotalStats() *Stats {
	total := &Stats{}
	for _, data := range d.Reporting {
		total.Likes += data.Likes
		total.Dislikes += data.Dislikes
		total.Comments += data.Comments
		total.Shares += data.Shares
		total.Views += data.Views
		total.Perks += data.Perks
		total.ApprovedClicks = append(total.ApprovedClicks, data.ApprovedClicks...)
		total.PendingClicks = append(total.PendingClicks, data.PendingClicks...)
		total.LegacyClicks += data.LegacyClicks
		total.Influencer += data.Influencer
		total.Agency += data.Agency
		total.Conversions = append(total.Conversions, data.Conversions...)
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
		data.Views += GetViews(likes, comments, shares)
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

func (d *Deal) Click(uuid string) {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	key := GetDate()
	data, ok := d.Reporting[key]
	if !ok {
		data = &Stats{}
		d.Reporting[key] = data
	}

	data.PendingClicks = append(data.PendingClicks, &Click{UUID: uuid, TS: int32(time.Now().Unix())})
}

func (d *Deal) AddConversions(convs []pixel.Conversion) {
	if d.Reporting == nil {
		d.Reporting = make(map[string]*Stats)
	}
	// key := GetDate()
	for _, conv := range convs {
		key := GetDateFromTime(time.Unix(conv.TS, 0))
		data, ok := d.Reporting[key]
		if !ok {
			data = &Stats{}
			d.Reporting[key] = data
		}

		data.Conversions = addConversion(data.Conversions, conv)
	}
}

func addConversion(conversions []pixel.Conversion, conv pixel.Conversion) []pixel.Conversion {
	shouldAdd := true
	for _, oldConv := range conversions {
		if oldConv.UUID == conv.UUID && oldConv.TS == conv.TS && oldConv.DealID == conv.DealID {
			// This is a dupe.. lets not add!
			shouldAdd = false
			break
		}
	}
	if shouldAdd {
		conversions = append(conversions, conv)
	}

	return conversions
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
			if stats.AgencyId != "" {
				data.AgencyId = stats.AgencyId
			}
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
		if stats.AgencyId != "" {
			data.AgencyId = stats.AgencyId
		}

		data.Likes += stats.Likes
		data.Dislikes += stats.Dislikes
		data.Comments += stats.Comments
		data.Shares += stats.Shares
		data.Views += stats.Views

		data.ApprovedClicks = append(data.ApprovedClicks, stats.ApprovedClicks...)
		data.LegacyClicks += stats.LegacyClicks

		data.Perks += stats.Perks

		data.Conversions = append(data.Conversions, stats.Conversions...)
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

func (d *Deal) IsActive() bool {
	return d.Assigned > 0 && d.Completed == 0 && d.InfluencerId != ""
}

func (d *Deal) IsComplete() bool {
	return d.Assigned > 0 && d.Completed > 0 && d.InfluencerId != ""
}

func (d *Deal) IsAvailable() bool {
	return d.Assigned == 0 && d.Completed == 0 && d.InfluencerId == ""
}

func (d *Deal) GetInstructions() []string {
	var instructions []string
	if d.ShortenedLink != "" {
		instructions = append(instructions, "Put this link in your bio/caption: "+d.ShortenedLink)
	}

	if len(d.Tags) > 0 {
		var tgPart string
		tgPart += "Hashtags to do: "
		for idx, tg := range d.Tags {
			if idx != 0 {
				tgPart += ", "
			}
			tgPart += "#" + tg
		}
		tgPart += ", #ad"
		instructions = append(instructions, tgPart)
	}

	if d.Mention != "" {
		instructions = append(instructions, "Mentions to do: "+d.Mention)
	}

	return instructions
}

func (d *Deal) ConvertToClear() *Deal {
	// Used to switch from ACTIVE deal to CLEAR deal
	d.InfluencerId = ""
	d.Assigned = 0
	d.Completed = 0
	d.Platforms = []string{}
	d.AssignedPlatform = ""
	d.Reporting = nil
	d.Perk = nil
	d.Spendable = 0
	d.LikelyEarnings = 0
	d.InfluencerName = ""

	return d
}

func (d *Deal) ConvertToActive() *Deal {
	// Used to switch from COMPLETED deal to ACTIVE deal
	d.Completed = 0
	d.PostUrl = ""
	d.AssignedPlatform = ""
	d.Tweet = nil
	d.YouTube = nil
	d.Facebook = nil
	d.Instagram = nil
	d.Reporting = nil

	return d
}

func (d *Deal) IsSubmitted() bool {
	if d.Submission == nil || !d.Submission.Approved {
		return false
	}

	return true
}

func (d *Deal) MatchesSubmission(caption string) bool {
	if d.Submission != nil {
		msg := strings.TrimSpace(strings.ToLower(d.Submission.Message))
		caption = strings.TrimSpace(strings.ToLower(caption))
		if strings.Contains(caption, msg) || strings.Contains(msg, caption) {
			return true
		}

		return false
	}

	return true
}

func GetAllDeals(db *bolt.DB, cfg *config.Config, active, complete bool) ([]*Deal, error) {
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

			if !cmp.IsValid() {
				return nil
			}

			for _, deal := range cmp.Deals {
				if deal.IsActive() && active {
					deals = append(deals, deal)
				}

				if deal.IsComplete() && complete {
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

func GetViews(likes, comments, shares int32) int32 {
	return int32(float64(likes+comments+shares) / engagementViewRatio)
}
