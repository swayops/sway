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
		auth: auth.New(db, cfg),
	}

	err := srv.InitializeDB(cfg)
	if err != nil {
		return nil, err
	}

	srv.InitializeRoutes(r)
	srv.InitializeChecks()

	return srv, nil
}

func (srv *Server) InitializeDB(cfg *config.Config) error {
	return srv.db.Update(func(tx *bolt.Tx) error {
		for _, val := range cfg.AllBuckets() {
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

//TODO should this be in the config?
var scopes = map[string]auth.ScopeMap{
	"talentAgency": {auth.TalentAgency: {Get: true, Post: true, Put: true, Delete: true}},
	"inf": {
		auth.TalentAgency: {Get: true, Post: true, Put: true, Delete: true},
		auth.Influencer:   {Get: true, Post: true, Put: true, Delete: true},
	},
	"adAgency": {auth.AdvertiserAgency: {Get: true, Post: true, Put: true, Delete: true}},
	"adv": {
		auth.AdvertiserAgency: {Get: true, Post: true, Put: true, Delete: true},
		auth.Advertiser:       {Get: true, Post: true, Put: true, Delete: true},
	},
}

func (srv *Server) InitializeRoutes(r *gin.Engine) {
	// Talent Agency
	createRoutes(r, srv, "/talentAgency", scopes["talentAgency"], auth.TalentAgencyItem, getTalentAgency, putTalentAgency, delTalentAgency)
	r.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))

	// AdAgency
	createRoutes(r, srv, "/adAgency", scopes["adAgency"], auth.AdvertiserAgencyItem, getAdAgency, putAdAgency, delAdAgency)
	r.GET("/getAllAdAgencies", getAllAdAgencies(srv))

	// Advertiser
	createRoutes(r, srv, "/advertiser", scopes["adv"], auth.AdvertiserItem, getAdvertiser, putAdvertiser, delAdvertiser)
	r.GET("/getAdvertisersByAgency/:id", getAdvertisersByAgency(srv))

	// Campaigns
	createRoutes(r, srv, "/campaign", scopes["adv"], auth.CampaignItem, getCampaign, putCampaign, delCampaign)

	sh := srv.auth.CheckScopes(scopes["adv"])
	oh := srv.auth.CheckOwnership(auth.CampaignItem, "campaignId")
	r.GET("/getCampaignsByAdvertiser/:id", srv.auth.VerifyUser, sh, getCampaignsByAdvertiser(srv))
	r.GET("/getCampaignAssignedDeals/:campaignId", srv.auth.VerifyUser, sh, oh, getCampaignAssignedDeals(srv))
	r.GET("/getCampaignCompletedDeals/:campaignId", srv.auth.VerifyUser, sh, oh, getCampaignCompletedDeals(srv))
	r.POST("/updateCampaign/:campaignId", srv.auth.VerifyUser, sh, oh, updateCampaign(srv))

	// Influencers
	createRoutes(r, srv, "/influencer", scopes["inf"], auth.InfluencerItem, getInfluencer, putInfluencer, delInfluencer)
	// TODO
	r.GET("/getInfluencersByCategory/:category", getInfluencersByCategory(srv))
	r.GET("/getInfluencersByAgency/:agencyId", getInfluencersByAgency(srv))
	r.GET("/setPlatform/:influencerId/:platform/:id", setPlatform(srv))
	r.GET("/setCategory/:influencerId/:category", setCategory(srv))
	r.GET("/getCategories", getCategories(srv))

	// Deal
	// TODO
	r.GET("/getDealsForInfluencer/:influencerId/:lat/:long", getDealsForInfluencer(srv))
	r.GET("/assignDeal/:influencerId/:campaignId/:dealId/:platform", assignDeal(srv))
	r.GET("/getDealsAssignedToInfluencer/:influencerId", getDealsAssignedToInfluencer(srv))
	r.GET("/unassignDeal/:influencerId/:campaignId/:dealId", unassignDeal(srv))
	// Offset in hours
	// TODO
	r.GET("/getDealsCompletedByInfluencer/:influencerId/:offset", getDealsCompletedByInfluencer(srv))
}

func (srv *Server) InitializeChecks() {
	newDealExplorer(srv)
	newStatsUpdate(srv)
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
