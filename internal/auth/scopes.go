package auth

type Scope string

const (
	AllScopes Scope = `*` // this is a special catch-all case for matching

	InvalidScope      Scope = ""
	AdminScope        Scope = `admin`
	AdAgencyScope     Scope = `advAgency`
	AdvertiserScope   Scope = `advertiser`
	TalentAgencyScope Scope = `talentAgency`
	InfluencerScope   Scope = `influencer`
)

func (s Scope) IsOneOf(os ...Scope) bool {
	for _, o := range os {
		if s == o {
			return true
		}
	}
	return false
}
func (s Scope) Valid() bool {
	switch s {
	case AdminScope, AdAgencyScope, TalentAgencyScope, AdvertiserScope, InfluencerScope:
		return true
	}
	return false
}

// CanCreate returns true if the current scope can create the specific user type
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

// CanOwn returns true if the current scope can create the specific item.
func (s Scope) CanOwn(it ItemType) bool {
	switch s {
	case AdminScope:
		return true
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
