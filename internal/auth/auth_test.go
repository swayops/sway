package auth

import "testing"

func TestScopes(t *testing.T) {
	var (
		sm = ScopeMap{
			Admin:            {true, true, true, true},
			AdvertiserAgency: {Post: true, Delete: true},
			AllUsers:         {Get: true},
		}

		tests = []struct {
			s  Scope
			m  string
			ex bool
		}{
			{Admin, "GET", true},
			{Admin, "invalid", false},
			{AdvertiserAgency, "POST", true},
			{AdvertiserAgency, "PUT", false},
			{AdvertiserAgency, "DELETE", true},
			{AdvertiserAgency, "GET", true}, // because it's inherited from AllUsers
			{Influencer, "GET", true},
			{Influencer, "POST", false},
			{TalentAgency, "GET", true},
			{TalentAgency, "POST", false},
		}
	)

	for _, ts := range tests {
		if v := sm.HasAccess(ts.s, ts.m); v != ts.ex {
			t.Errorf("wanted %+v, got %v", ts, v)
		}
	}
}
