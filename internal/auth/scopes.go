package auth

type Scope string

const (
	Admin            Scope = `admin`
	TalentAgency     Scope = `talentAgency`
	AdvertiserAgency Scope = `advertiserAgency`
	Influencer       Scope = `influencer`

	AllUsers Scope = `*` // this is a special case to allow public access
)

func (s Scope) Valid() bool {
	switch s {
	case Admin, TalentAgency, AdvertiserAgency, Influencer:
		return true
	}
	return false
}

type ScopeMap map[Scope]struct{ Get, Put, Post, Delete bool }

func (sm ScopeMap) HasAccess(scope Scope, method string) bool {
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
