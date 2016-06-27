package server

func notify(s *Server) error {
	for _, inf := range getAllInfluencers(s, false) {
		inf.GetAvailableDeals(s.Campaigns, s.budgetDb, "", nil, false, s.Cfg)
	}
	return nil
}
