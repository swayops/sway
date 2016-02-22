package server

import (
	"fmt"
	"log"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/misc"
)

type Server struct {
	Cfg  *config.Config
	r    *gin.Engine
	db   *bolt.DB
	auth *auth.Auth
	// Db
}

func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)

	srv := &Server{
		Cfg:  cfg,
		r:    r,
		db:   db,
		auth: auth.New(db, cfg, "/login"),
	}

	srv.InitializeDB(cfg)
	srv.InitializeRoutes(r)
	return srv, nil
}

func (srv *Server) InitializeDB(cfg *config.Config) error {
	return srv.db.Update(func(tx *bolt.Tx) error {
		for _, val := range cfg.Bucket.All {
			log.Println("Initializing bucket", val)
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
	r.GET("/getCampaignAssignedDeals/:campaignId", getCampaignAssignedDeals(srv))
	r.GET("/getCampaignCompletedDeals/:campaignId", getCampaignCompletedDeals(srv))
	r.GET("/getCampaignAuditedDeals/:campaignId", getCampaignAuditedDeals(srv))
	r.GET("/campaignStatus/:campaignId/:status", toggleCampaignStatus(srv))
	r.POST("/updateCampaignGeo/:campaignId", updateCampaignGeo(srv))

	// Groups
	createRoutes(r, srv, "/group", getGroup, putGroup, delGroup)
	r.GET("/getGroupsByAgency/:id", getGroupByAgency(srv))
	r.GET("/getAllGroups", getAllGroups(srv))

	// Influencers
	createRoutes(r, srv, "/influencer", getInfluencer, putInfluencer, delInfluencer)
	r.GET("/getInfluencersByGroup/:id", getInfluencersByGroup(srv))
	r.GET("/addInfluencerToGroup/:influencerId/:groupId", addInfluencerToGroup(srv))
	r.GET("/delInfluencerFromGroup/:influencerId/:groupId", delInfluencerFromGroup(srv))
	r.GET("/setPlatform/:influencerId/:platform/:id", setPlatform(srv))

	// Deal
	r.GET("/getDealsForInfluencer/:influencerId/:lat/:long", getDealsForInfluencer(srv))
	r.GET("/assignDeal/:influencerId/:campaignId/:dealId", assignDeal(srv))
	r.GET("/getDealsAssignedToInfluencer/:influencerId", getDealsAssignedToInfluencer(srv))
	r.GET("/unassignDeal/:influencerId/:campaignId/:dealId", unassignDeal(srv))
}

func (srv *Server) Run() (err error) {
	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		err = srv.r.Run(":" + srv.Cfg.Port)
		wg.Done()
	}()

	wg.Wait()
	return
}
