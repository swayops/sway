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
}

func (srv *Server) InitializeInfluencers() {

}
