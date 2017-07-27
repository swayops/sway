package influencer

import "sync"

type Influencers struct {
	mux      sync.RWMutex
	store    map[string]Influencer
	counts   map[string]int64
	avgYield float64
}

func NewInfluencers() *Influencers {
	return &Influencers{
		store: make(map[string]Influencer),
		// Stores counts for talent agency ID to influencer count
		counts: make(map[string]int64),
	}
}

func (p *Influencers) Set(infs []Influencer) {
	store := make(map[string]Influencer)
	counts := make(map[string]int64)
	var totalYields float64
	for _, inf := range infs {
		store[inf.Id] = inf
		counts[inf.AgencyId] += 1

		totalYields += GetMaxYield(nil, inf.YouTube, inf.Facebook, inf.Twitter, inf.Instagram)
	}

	// Lets also set avg yield value for our reporting purposes
	p.mux.Lock()
	p.avgYield = totalYields / float64(len(infs))
	p.store = store
	p.counts = counts
	p.mux.Unlock()
}

func (p *Influencers) SetInfluencer(id string, inf Influencer) {
	p.mux.Lock()
	p.store[id] = inf
	p.counts[inf.AgencyId] += 1
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

func (p *Influencers) GetAvgYield() float64 {
	var avgYield float64
	p.mux.RLock()
	avgYield = p.avgYield
	p.mux.RUnlock()
	return avgYield
}

func (p *Influencers) GetAllEmails() map[string]bool {
	store := make(map[string]bool)
	p.mux.RLock()
	for _, inf := range p.store {
		store[inf.EmailAddress] = true
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

func (p *Influencers) GetCount(id string) int64 {
	p.mux.RLock()
	val, _ := p.counts[id]
	p.mux.RUnlock()
	return val
}

func (p *Influencers) Len() int {
	p.mux.RLock()
	l := len(p.store)
	p.mux.RUnlock()
	return l
}
