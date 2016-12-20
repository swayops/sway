package common

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/geo"
)

type Campaign struct {
	Id   string `json:"id"` // Should not passed for putCampaign
	Name string `json:"name"`

	CreatedAt int64 `json:"createdAt"`

	Budget float64 `json:"budget"` // Always monthly

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

	// Categories the client is targeting
	Categories []string `json:"categories,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`

	Perks *Perk `json:"perks,omitempty"`

	Whitelist map[string]bool `json:"whitelist,omitempty"` // List of emails
	// Copied from advertiser
	Blacklist map[string]bool `json:"blacklist,omitempty"`

	// Internal attribute set by putCampaign and un/assignDeal
	// Contains all the deals sent out by this campaign.. keyed off of deal ID
	Deals map[string]*Deal `json:"deals,omitempty"`
}

func (cmp *Campaign) IsValid() bool {
	return cmp.Budget > 0 && len(cmp.Deals) > 0 && cmp.Status && cmp.Approved > 0
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
