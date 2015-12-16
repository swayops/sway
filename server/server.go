package server

import (
	"fmt"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

var buckets = []string{"test"}

type Server struct {
	cfg *config.Config
	r   *gin.Engine
	db  *bolt.DB
	// Db
}

func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)

	srv := &Server{
		cfg: cfg,
		r:   r,
		db:  db,
	}

	srv.InitializeDB(cfg.Buckets)
	srv.InitializeRoutes(r)
	return srv, nil
}

func (srv *Server) InitializeDB(buckets []string) error {
	return srv.db.Update(func(tx *bolt.Tx) error {
		for _, val := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(val)); err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			if err := misc.InitIndex(tx, val, 1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (srv *Server) InitializeRoutes(r *gin.Engine) {
	// RTB Handlers //
	// Agency
	createRoutes(r, srv, "/agency", getAgency, putAgency, delAgency)
	r.GET("/getAllAgencies", getAllAgencies(srv))

	// Advertiser
	createRoutes(r, srv, "/advertiser", getAdvertiser, putAdvertiser, delAdvertiser)
	r.GET("/getAdvertisersByAgency/:id", getAdvertisersByAgency(srv))

	// Campaigns
	createRoutes(r, srv, "/campaign", getCampaign, putCampaign, delCampaign)
	r.GET("/getCampaignsByAdvertiser/:id", getCampaignsByAdvertiser(srv))

	// Deal
	// createRoutes(r, srv, "/deal", getDeal, putDeal, delDeal)

	// Groups
	createRoutes(r, srv, "/group", getGroup, putGroup, delGroup)
	r.GET("/getGroupsByAgency/:id", getGroupByAgency(srv))
	r.GET("/getAllGroups", getAllGroups(srv))

	// Influencers
	// r.GET("/getInfluencerByAgency/:id", getInfluencerByAgency(srv))
	// r.GET("/getInfluencersByGroup/:id", getInfluencerByGroup(srv))
}

func (srv *Server) Run() (err error) {
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		err = srv.r.Run(":" + srv.cfg.Port)
		wg.Done()
	}()

	wg.Wait()
	return
}
