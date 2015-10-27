package server

import "github.com/swayops/internal/config"

type Server struct {
	Cfg *config.Config
	// Db
}

func New(cfg *config.Config) (*Server, error) {
	srv := &Server{
		Cfg: cfg,
	}
	srv.InitializeInfluencers()
	srv.InitializeCampaigns()
}

func (srv *Server) InitializeInfluencers() {
	// Load influencers from DB
}

func (srv *Server) InitializeCampaigns() {
	// Load campaigns from DB
}
