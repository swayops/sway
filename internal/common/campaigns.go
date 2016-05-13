package common

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

type Campaign struct {
	Id   string `json:"id"` // Should not passed for putCampaign
	Name string `json:"name"`

	Budget float32 `json:"budget"` // Always monthly

	AdvertiserId string `json:"advertiserId"`
	AgencyId     string `json:"agencyId"`

	Active bool `json:"active"`

	// Social Media Post/User Requirements
	Tags    []string          `json:"hashtags,omitempty"`
	Mention string            `json:"mention,omitempty"`
	Link    string            `json:"link,omitempty"`
	Task    string            `json:"task,omitempty"`
	Geos    []*misc.GeoRecord `json:"geos,omitempty"`   // Geos the campaign is targeting
	Gender  string            `json:"gender,omitempty"` // "m" or "f" or "mf"

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`

	// Categories the client is targeting
	Categories []string `json:"categories,omitempty"`

	// White/Blacklist influencers
	Whitelist *TargetList `json:"whitelist,omitempty"`
	Blacklist *TargetList `json:"blacklist,omitempty"`

	Perks string `json:"perks,omitempty"` // Perks need to be specced out

	// Internal attribute set by putCampaign and un/assignDeal
	// Contains all the deals sent out by this campaign.. keyed off of deal ID
	Deals map[string]*Deal `json:"deals,omitempty"`
}

type TargetList struct {
	Twitter   []string `json:"twitter,omitempty"`
	Facebook  []string `json:"facebook,omitempty"`
	Instagram []string `json:"instagram,omitempty"`
	YouTube   []string `json:"youtube,omitempty"`
}

func (tl *TargetList) Sanitize() *TargetList {
	tl.Twitter = lowerArr(tl.Twitter)
	tl.Facebook = lowerArr(tl.Facebook)
	tl.Instagram = lowerArr(tl.Instagram)
	tl.YouTube = lowerArr(tl.YouTube)
	return tl
}

func (cmp *Campaign) IsValid() bool {
	return cmp.Active && cmp.Budget > 0 && len(cmp.Deals) > 0
}

type Campaigns struct {
	mux   sync.RWMutex
	store map[string]*Campaign
}

func NewCampaigns() *Campaigns {
	return &Campaigns{
		store: make(map[string]*Campaign),
	}
}

func (p *Campaigns) Set(db *bolt.DB, cfg *config.Config) {
	cmps := GetAllActiveCampaigns(db, cfg)
	p.mux.Lock()
	p.store = cmps
	p.mux.Unlock()
}

func (p *Campaigns) SetCampaign(id string, cmp *Campaign) {
	p.mux.Lock()
	p.store[id] = cmp
	p.mux.Unlock()
}

func (p *Campaigns) GetStore() map[string]*Campaign {
	store := make(map[string]*Campaign)
	p.mux.RLock()
	for cId, cmp := range p.store {
		store[cId] = cmp
	}
	p.mux.RUnlock()
	return store
}

func (p *Campaigns) Get(id string) (*Campaign, bool) {
	p.mux.RLock()
	val, ok := p.store[id]
	p.mux.RUnlock()
	return val, ok
}

func GetAllActiveCampaigns(db *bolt.DB, cfg *config.Config) map[string]*Campaign {
	// Returns a list of active campaign IDs in the system
	campaignList := make(map[string]*Campaign)

	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
			cmp := &Campaign{}
			if err := json.Unmarshal(v, cmp); err != nil {
				log.Println("error when unmarshalling campaign", string(v))
				return nil
			}
			if cmp.IsValid() {
				campaignList[cmp.Id] = cmp
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

func lowerArr(s []string) []string {
	for i, v := range s {
		s[i] = strings.ToLower(v)
	}
	return s
}
