package server

func notify(s *Server) error {
	for _, inf := range getAllInfluencers(s) {
		inf.GetAvailableDeals(s.Campaigns, s.db, s.budgetDb, "", nil, false, s.Cfg)
	}
	return nil
}
