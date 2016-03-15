package auth

type Scope string

const (
	Admin            Scope = `admin`
	AdvertiserAgency Scope = `advAgency`
	Advertiser       Scope = `advertiser`
	TalentAgency     Scope = `talentAgency`
	Influencer       Scope = `influencer`

	AllUsers Scope = `*` // this is a special catch-all case for matching
)

func (s Scope) Valid() bool {
	switch s {
	case Admin, AdvertiserAgency, TalentAgency, Advertiser, Influencer:
		return true
	}
	return false
}

func (s Scope) CanCreate(child Scope) bool {
	switch s {
	case Admin:
		return true
	case AdvertiserAgency:
		return child == Advertiser
	case TalentAgency:
		return child == Influencer
	}
	return false
}

func (s Scope) CanOwn(it ItemType) bool {
	switch s {
	case Admin:
		return it == AdvertiserAgencyItem || it == TalentAgencyItem
	case AdvertiserAgency:
		return it == AdvertiserItem
	case TalentAgency:
		return it == InfluencerItem
	case Advertiser:
		return it == CampaignItem
	}
	return false
}

type ScopeMap map[Scope]struct{ Get, Put, Post, Delete bool }

func (sm ScopeMap) HasAccess(scope Scope, method string) bool {
	if scope == Admin {
		return true
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
	if !v && scope != AllUsers {
		v = sm.HasAccess(AllUsers, method)
	}
	return v
}
