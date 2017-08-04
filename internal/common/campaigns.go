package common

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/misc"
)

type Campaign struct {
	Id   string `json:"id"` // Should not passed for putCampaign
	Name string `json:"name"`

	CreatedAt int64 `json:"createdAt"`

	Budget  float64 `json:"budget"`
	Monthly bool    `json:"monthly"` // Is this an ongoing monthly campaign?

	TermsAndConditions string `json:"terms"`

	AdvertiserId string `json:"advertiserId"`
	AgencyId     string `json:"agencyId"`

	// Image URL for the campaign
	ImageURL  string `json:"imageUrl"`
	ImageData string `json:"imageData,omitempty"` // this is input-only and never saved to the db

	Company string `json:"company,omitempty"`

	Status   bool  `json:"status"`
	Approved int32 `json:"approved"` // Set to ts when admin receives all perks (or there are no perks)

	// Social Media Post/User Requirements
	Tags    []string         `json:"tags,omitempty"`
	Mention string           `json:"mention,omitempty"`
	Link    string           `json:"link,omitempty"`
	Task    string           `json:"task,omitempty"`
	Geos    []*geo.GeoRecord `json:"geos,omitempty"` // Geos the campaign is targeting
	Male    bool             `json:"male,omitempty"`
	Female  bool             `json:"female,omitempty"`

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`

	// Only allow brand safe influencers?
	BrandSafe bool `json:"brandSafe,omitempty"`

	// Categories the client is targeting
	Categories []string `json:"categories,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Audiences  []string `json:"audiences,omitempty"` // Audience IDs the client is targeting

	FollowerTarget *Range      `json:"followerTarget,omitempty"` // Min and max followers this campaign is targeting
	EngTarget      *Range      `json:"engTarget,omitempty"`      // Min and max engagements this campaign is targeting
	PriceTarget    *FloatRange `json:"priceTarget,omitempty"`    // Min and max payouts this campaign is targeting

	Perks *Perk `json:"perks,omitempty"`

	LegacyWhitelist map[string]bool `json:"whitelist,omitempty"` // List of emails

	Whitelist map[string]*Range `json:"whitelistSchedule,omitempty"` // List of emails and their schedules
	// Copied from advertiser
	AdvertiserBlacklist map[string]bool `json:"blacklist,omitempty"`
	CampaignBlacklist   map[string]bool `json:"cmpBlacklist,omitempty"`

	// Internal attribute set by putCampaign and un/assignDeal
	// Contains all the deals sent out by this campaign.. keyed off of deal ID
	Deals map[string]*Deal `json:"deals,omitempty"`
	Plan  int              `json:"planID,omitempty"`

	Timeline []*Timeline `json:"timeline,omitempty"`

	RequiresSubmission bool `json:"reqSub,omitempty"` // Does the advertiser require submission?
}

type Range struct {
	From int64 `json:"from,omitempty"`
	To   int64 `json:"to,omitempty"`
}

func (r *Range) InRange(val int64) bool {
	if r.From == 0 && r.To == 0 {
		return true
	}

	if val < r.From || val > r.To {
		return false
	}

	return true
}

type FloatRange struct {
	From float64 `json:"from,omitempty"`
	To   float64 `json:"to,omitempty"`
}

func (r *FloatRange) InRange(val float64) bool {
	if r.From == 0 && r.To == 0 {
		return true
	}

	if val < r.From || val > r.To {
		return false
	}

	return true
}

func (cmp *Campaign) IsValid() bool {
	return (cmp.Budget > 0 || cmp.IsProductBasedBudget()) && len(cmp.Deals) > 0 && cmp.Status && cmp.Approved > 0
}

func (cmp *Campaign) IsProductBasedBudget() bool {
	return cmp.Budget == 0 && cmp.Perks != nil
}

func (cmp *Campaign) HasMailedPerk() bool {
	for _, deal := range cmp.Deals {
		if deal.Perk != nil && deal.Perk.Status {
			return true
		}
	}
	return false
}

func (cmp *Campaign) HasAcceptedDeal() bool {
	for _, deal := range cmp.Deals {
		if deal.Completed == 0 && deal.Assigned > 0 {
			return true
		}
	}
	return false
}

func (cmp *Campaign) HasCompletedDeal() bool {
	for _, deal := range cmp.Deals {
		if deal.Completed > 0 {
			return true
		}
	}
	return false
}

const (
	WIKI = "https://swayops.com/wiki/how-sway-works.php"
)

func (cmp *Campaign) AddToTimeline(msg string, unique bool, cfg *config.Config) {
	// If the unique flag is present we will make sure this msg
	// has not previously been set
	tl := &Timeline{Message: msg, TS: time.Now().Unix()}

	editCampaign := fmt.Sprintf("/editCampaign/%s/%s", cmp.AdvertiserId, cmp.Id)
	contentFeed := fmt.Sprintf("/contentFeed/%s", cmp.AdvertiserId)
	manageCampaigns := fmt.Sprintf("/mCampaigns/%s", cmp.AdvertiserId)
	shippingInfo := fmt.Sprintf("/shippingPerks/%s", cmp.AdvertiserId)

	switch msg {
	case PERK_WAIT:
		tl.Link = shippingInfo
	case CAMPAIGN_START, PERKS_RECEIVED:
		tl.Link = WIKI
	case DEAL_ACCEPTED, PERKS_MAILED:
		tl.Link = manageCampaigns
	case CAMPAIGN_SUCCESS:
		tl.Link = contentFeed
	case CAMPAIGN_PAUSED:
		tl.Link = editCampaign
	}

	if len(cmp.Timeline) == 0 {
		cmp.Timeline = []*Timeline{tl}
	} else {
		if unique {
			for _, old := range cmp.Timeline {
				if old.Message == tl.Message {
					// If this message has been made before.. bail!
					return
				}
			}
		}
		cmp.Timeline = append(cmp.Timeline, tl)
	}
}

func (cmp *Campaign) GetEmptyDeals() int32 {
	// Gets number of deals that are empty
	var empty int32
	for _, deal := range cmp.Deals {
		if deal.IsAvailable() {
			empty += 1
		}
	}
	return empty
}

func (cmp *Campaign) GetPendingDetails() (float64, int) {
	// Lets figure out the amount of expected spend remaining
	// and the number of empty deals
	var (
		pendingSpend float64
		dealsEmpty   int
	)

	for _, deal := range cmp.Deals {
		if deal.IsActive() {
			// Value is set when the influencer was first
			// offered the deal.. we assume they will complete
			// it and reach their likely earnings!
			pendingSpend += deal.LikelyEarnings
		} else if deal.IsComplete() && misc.WithinLast(deal.Completed, 6) {
			// If the deal completed within the last 6 hours.. the post
			// will not have reached it's full potential yet.. so the target
			// yield may be artificially inflated for a while (until the post reaches
			// its avg engagements). As a result of the inflation, we'll have one less empty
			// deal and the SAME spendable.. so for the first 6 hours lets subtract influencer's full
			// likely earnings value
			pendingSpend += deal.LikelyEarnings
		} else if deal.IsAvailable() {
			dealsEmpty += 1
		}
	}

	return pendingSpend, dealsEmpty
}

func (cmp *Campaign) GetTargetYield(spendable float64) (float64, float64) {
	// Lets figure out the number of available deals AND the approximate budget
	// that is used up

	pendingSpend, dealsEmpty := cmp.GetPendingDetails()
	if dealsEmpty == 0 {
		return 0, 0
	}

	// Lets subtract the pending spend that will come in soon from
	// active deals
	filteredSpendable := spendable - pendingSpend
	if filteredSpendable < 0 {
		return 0, 0
	}

	// For even distribution.. lets give a target spendable for each available
	// deal
	target := filteredSpendable / float64(dealsEmpty)

	// 40% margin left and right of target
	var margin float64
	if (float64(dealsEmpty) / float64(len(cmp.Deals))) > 0.7 {
		// If less than 30% of the campaign has been filled.. lets be a bit more lenient
		margin = 0.6 * target
	} else {
		margin = 0.40 * target
	}
	return target - margin, target + margin
}

type Campaigns struct {
	mux       sync.RWMutex
	store     map[string]Campaign
	activeAdv map[string]bool
	activeAg  map[string]bool
}

func NewCampaigns() *Campaigns {
	return &Campaigns{
		store: make(map[string]Campaign),
	}
}

func (p *Campaigns) Set(db *bolt.DB, cfg *config.Config, adv, ag map[string]bool) {
	cmps := getAllActiveCampaigns(db, cfg, adv, ag)
	p.mux.Lock()
	p.store = cmps
	p.activeAdv = adv
	p.activeAg = ag
	p.mux.Unlock()
}

func (p *Campaigns) SetActiveCampaign(id string, cmp Campaign) {
	// Verifies whether the campaign is active
	p.mux.Lock()

	activeAdv := p.activeAdv[cmp.AdvertiserId]
	activeAg := p.activeAg[cmp.AgencyId]
	if cmp.IsValid() && activeAdv && activeAg {
		p.store[id] = cmp
	}

	p.mux.Unlock()
}

func (p *Campaigns) SetCampaign(id string, cmp Campaign) {
	p.mux.Lock()
	p.store[id] = cmp
	p.mux.Unlock()
}

func (p *Campaigns) GetCampaignAsStore(cid string) map[string]Campaign {
	// Override used when you want just one campaign as a store
	// Made for influencer.GetAvailableDeals
	store := make(map[string]Campaign)

	p.mux.RLock()
	val, ok := p.store[cid]
	if ok {
		store[cid] = val
	}
	p.mux.RUnlock()

	return store
}

func (p *Campaigns) GetStore() map[string]Campaign {
	store := make(map[string]Campaign)
	p.mux.RLock()
	for cId, cmp := range p.store {
		store[cId] = cmp
	}
	p.mux.RUnlock()
	return store
}

func (p *Campaigns) GetAvailableDealCount() int32 {
	var count int32
	p.mux.RLock()
	for _, cmp := range p.store {
		count += cmp.GetEmptyDeals()
	}
	p.mux.RUnlock()
	return count
}

func (p *Campaigns) Get(id string) (Campaign, bool) {
	p.mux.RLock()
	val, ok := p.store[id]
	p.mux.RUnlock()
	return val, ok
}

func (p *Campaigns) Len() int {
	p.mux.RLock()
	l := len(p.store)
	p.mux.RUnlock()
	return l
}

func getAllActiveCampaigns(db *bolt.DB, cfg *config.Config, adv, ag map[string]bool) map[string]Campaign {
	// Returns a list of active campaign IDs in the system
	campaignList := make(map[string]Campaign)

	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &Campaign{}
			if err := json.Unmarshal(v, cmp); err != nil {
				log.Printf("error when unmarshalling campaign %s: %v", v, err)
				return nil
			}

			activeAdv := adv[cmp.AdvertiserId]
			activeAg := ag[cmp.AgencyId]
			if cmp.IsValid() && activeAdv && activeAg {
				campaignList[cmp.Id] = *cmp
			}

			return
		})
		return nil
	}); err != nil {
		log.Println("Err getting all active campaigns", err)
	}

	return campaignList
}
func GetCampaign(cid string, db *bolt.DB, cfg *config.Config) *Campaign {
	var (
		v   []byte
		err error
		g   Campaign
	)

	if err := db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.Bucket.Campaign)).Get([]byte(cid))
		return nil
	}); err != nil {
		return nil
	}

	if err = json.Unmarshal(v, &g); err != nil {
		return nil
	}

	return &g
}
