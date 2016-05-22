package server

import "github.com/swayops/sway/internal/influencer"

func notify(s *Server) error {
	influencers := influencer.GetAllInfluencers(s.db, s.Cfg)
	for _, inf := range influencers {
		_ = influencer.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, inf.Id, "", nil, false, s.Cfg)

	}
	return nil
}
