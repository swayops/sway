package common

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
)

type Audience struct {
	Id   string `json:"id"` // Set internally
	Name string `json:"name"`

	Members map[string]bool `json:"members"` // List of email addresses

	// Image URL for the audience
	ImageURL  string `json:"imageUrl"`
	ImageData string `json:"imageData,omitempty"` // this is input-only and never saved to the db

}

type Audiences struct {
	mux   sync.RWMutex
	store map[string]Audience
}

func NewAudiences() *Audiences {
	return &Audiences{
		store: make(map[string]Audience),
	}
}

func (p *Audiences) Set(db *bolt.DB, cfg *config.Config) error {
	aud, err := GetAudience(db, cfg)
	if err != nil {
		return err
	}

	p.mux.Lock()
	p.store = aud
	p.mux.Unlock()

	return nil
}

func (p *Audiences) SetAudience(ID string, aud Audience) error {
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

func (p *Audiences) GetStore(ID string) map[string]Audience {
	store := make(map[string]Audience)
	p.mux.RLock()
	for audID, aud := range p.store {
		if ID == "" || ID == aud.Id {
			store[audID] = aud
		}
	}
	p.mux.RUnlock()
	return store
}

func GetAudience(db *bolt.DB, cfg *config.Config) (map[string]Audience, error) {
	audiences := make(map[string]Audience)
	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Audience)).ForEach(func(k, v []byte) (err error) {
			var aud Audience
			if err := json.Unmarshal(v, &aud); err != nil {
				log.Println("error when unmarshalling audience", string(v))
				return nil
			}

			audiences[aud.Id] = aud
			return
		})
		return nil
	}); err != nil {
		return audiences, err
	}

	return audiences, nil
}
