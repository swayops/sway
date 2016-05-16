package auth

type Scope string

const (
	AdminScope        Scope = `admin`
	AdAgencyScope     Scope = `advAgency`
	AdvertiserScope   Scope = `advertiser`
	TalentAgencyScope Scope = `talentAgency`
	InfluencerScope   Scope = `influencer`

	AllScopes Scope = `*` // this is a special catch-all case for matching
)

func (s Scope) Valid() bool {
	switch s {
	case AdminScope, AdAgencyScope, TalentAgencyScope, AdvertiserScope, InfluencerScope:
		return true
	}
	return false
}

func (s Scope) CanCreate(child Scope) bool {
	switch s {
	case AdminScope:
		return true
	case AdAgencyScope:
		return child == AdvertiserScope
	case TalentAgencyScope:
		return child == InfluencerScope
	}
	return false
}

func (s Scope) CanOwn(it ItemType) bool {
	switch s {
	case AdminScope:
		return it == AdAgencyItem || it == TalentAgencyItem
	case AdAgencyScope:
		return it == AdvertiserItem
	case TalentAgencyScope:
		return it == InfluencerItem
	case AdvertiserScope:
		return it == CampaignItem
	}
	return false
}

type ScopeMap map[Scope]struct{ Get, Put, Post, Delete bool }

func (sm ScopeMap) HasAccess(scope Scope, method string) bool {
	if scope == AdminScope {
		return true
	} else if sm == nil {
		return false
	}

	var v bool
	if m, ok := sm[scope]; ok {
		switch method {
		case "HEAD", "GET":
			v = m.Get
		case "PUT":
			v = m.Put
		case "POST":
			v = m.Post
		case "DELETE":
			v = m.Delete
		}
	}
	if !v && scope != AllScopes {
		v = sm.HasAccess(AllScopes, method)
	}
	return v
}
