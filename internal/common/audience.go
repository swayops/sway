package common

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
)

type Audience struct {
	Id   string `json:"id"` // Set internally
	Name string `json:"name"`

	Members   map[string]bool `json:"members"` // List of email addresses
	Followers int64           `json:"followers"`

	// Image URL for the audience
	ImageURL  string `json:"imageUrl"`
	ImageData string `json:"imageData,omitempty"` // this is input-only and never saved to the db

}

type Audiences struct {
	mux   sync.RWMutex
	store map[string]*Audience
}

func NewAudiences() *Audiences {
	return &Audiences{
		store: make(map[string]*Audience),
	}
}

func (p *Audiences) Set(db *bolt.DB, cfg *config.Config, byEmail map[string]int64) error {
	aud, err := GetAudience(db, cfg, byEmail)
	if err != nil {
		return err
	}

	p.mux.Lock()
	p.store = aud
	p.mux.Unlock()

	return nil
}

func (p *Audiences) SetAudience(ID string, aud *Audience) error {
	p.mux.Lock()
	p.store[ID] = aud
	p.mux.Unlock()

	return nil
}

func (p *Audiences) IsAllowed(id, email string) bool {
	var allowed bool

	p.mux.RLock()
	val, ok := p.store[id]
	if ok {
		_, allowed = val.Members[email]
	}
	p.mux.RUnlock()
	return allowed
}

func (p *Audiences) GetStore(ID string) map[string]*Audience {
	store := make(map[string]*Audience)
	p.mux.RLock()
	for audID, aud := range p.store {
		if ID == "" || ID == aud.Id {
			store[audID] = aud
		}
	}
	p.mux.RUnlock()
	return store
}

func (p *Audiences) GetStoreByAdvertiser(advID string) map[string]*Audience {
	store := make(map[string]*Audience)
	p.mux.RLock()
	for audID, aud := range p.store {
		if strings.HasPrefix(audID, advID+"_") {
			store[audID] = aud
		}
	}
	p.mux.RUnlock()
	return store
}

func (p *Audiences) Delete(ID string) {
	p.mux.Lock()
	delete(p.store, ID)
	p.mux.Unlock()
}

func GetAudience(db *bolt.DB, cfg *config.Config, byEmail map[string]int64) (map[string]*Audience, error) {
	audiences := make(map[string]*Audience)
	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Audience)).ForEach(func(k, v []byte) (err error) {
			var aud Audience
			if err := json.Unmarshal(v, &aud); err != nil {
				log.Println("error when unmarshalling audience", string(v))
				return nil
			}

			var followers int64
			for email, _ := range aud.Members {
				tmpFollowers, ok := byEmail[email]
				if ok {
					followers += tmpFollowers
				}
			}

			aud.Followers = followers
			audiences[aud.Id] = &aud
			return
		})
		return nil
	}); err != nil {
		return audiences, err
	}

	return audiences, nil
}
