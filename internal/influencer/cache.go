package influencer

import "sync"

type Influencers struct {
	mux   sync.RWMutex
	store map[string]Influencer
}

func NewInfluencers() *Influencers {
	return &Influencers{
		store: make(map[string]Influencer),
	}
}

func (p *Influencers) Set(infs []Influencer) {
	store := make(map[string]Influencer)
	for _, inf := range infs {
		store[inf.Id] = inf
	}
	p.mux.Lock()
	p.store = store
	p.mux.Unlock()
}

func (p *Influencers) SetInfluencer(id string, inf Influencer) {
	p.mux.Lock()
	p.store[id] = inf
	p.mux.Unlock()
}

func (p *Influencers) GetAll() map[string]Influencer {
	store := make(map[string]Influencer)
	p.mux.RLock()
	for infId, inf := range p.store {
		store[infId] = inf
	}
	p.mux.RUnlock()
	return store
}

func (p *Influencers) GetAllIDs() []string {
	p.mux.RLock()
	store := make([]string, 0, len(p.store))
	for infId, _ := range p.store {
		store = append(store, infId)
	}
	p.mux.RUnlock()
	return store
}

func (p *Influencers) Get(id string) (Influencer, bool) {
	p.mux.RLock()
	val, ok := p.store[id]
	p.mux.RUnlock()
	return val, ok
}

func (p *Influencers) Len() int {
	p.mux.RLock()
	l := len(p.store)
	p.mux.RUnlock()
	return l
}
