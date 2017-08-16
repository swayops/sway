package influencer

import (
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
)

type Scraps struct {
	mux   sync.RWMutex
	store map[string]Scrap
}

func NewScraps() *Scraps {
	return &Scraps{
		store: make(map[string]Scrap),
	}
}

func (p *Scraps) Set(db *bolt.DB, cfg *config.Config, scraps map[string]Scrap) error {
	p.mux.Lock()
	p.store = scraps
	p.mux.Unlock()

	return nil
}

func (p *Scraps) SetScrap(id string, scrap Scrap) {
	p.mux.Lock()
	p.store[id] = scrap
	p.mux.Unlock()
}

func (p *Scraps) Get(id string) (Scrap, bool) {
	p.mux.Lock()
	scrap, ok := p.store[id]
	p.mux.Unlock()

	return scrap, ok
}

func (p *Scraps) GetKeywords(email, id string, sandbox bool) (keywords []string) {
	p.mux.RLock()
	for _, sc := range p.store {
		if strings.EqualFold(sc.EmailAddress, email) && strings.EqualFold(sc.Name, id) && len(sc.Keywords) > 0 {
			keywords = sc.Keywords
			if sandbox {
				// Lets prepend everything with a prefix so we know where the kw is coming
				// from in tests
				keywords = prepend(sc.Keywords)
			}
			break
		}
	}
	p.mux.RUnlock()
	return
}

func (p *Scraps) GetStore() map[string]Scrap {
	store := make(map[string]Scrap)
	p.mux.RLock()
	for k, v := range p.store {
		store[k] = v
	}
	p.mux.RUnlock()
	return store
}
