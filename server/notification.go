package server

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
)

func notify(s *Server) error {
	var influencers []*auth.Influencer
	s.db.View(func(tx *bolt.Tx) error {
		return s.auth.GetUsersByTypeTx(tx, auth.InfluencerScope, func(u *auth.User) error {
			if inf := auth.GetInfluencer(u); inf != nil {
				influencers = append(influencers, inf)
			}
			return nil
		})
	})
	for _, inf := range influencers {
		inf.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, "", nil, false, s.Cfg)
	}
	return nil
}
