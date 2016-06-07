package auth

import "testing"

func TestScopes(t *testing.T) {
	var (
		sm = ScopeMap{
			AdminScope:    {true, true, true, true},
			AdAgencyScope: {Post: true, Delete: true},
			AllScopes:     {Get: true},
		}

		tests = []struct {
			s  Scope
			m  string
			ex bool
		}{
			{AdminScope, "GET", true},
			{AdAgencyScope, "POST", true},
			{AdAgencyScope, "PUT", false},
			{AdAgencyScope, "DELETE", true},
			{AdAgencyScope, "GET", true}, // because it's inherited from AllUsers
			{InfluencerScope, "GET", true},
			{InfluencerScope, "POST", false},
			{TalentAgencyScope, "GET", true},
			{TalentAgencyScope, "POST", false},
		}
	)

	for _, ts := range tests {
		if v := sm.HasAccess(ts.s, ts.m); v != ts.ex {
			t.Errorf("wanted %+v, got %v", ts, v)
		}
	}
}
