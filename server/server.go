package server

import (
	"fmt"
	"log"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

const (
	adminEmail  = "admin@swayops.com"
	adminPass   = "Rf_jv9hM3-"
	agencyEmail = "agency@swayops.com"
	agencyPass  = "Rf_jv9hM4-"
)

type Server struct {
	Cfg         *config.Config
	r           *gin.Engine
	db          *bolt.DB
	budgetDb    *bolt.DB
	reportingDb *bolt.DB
	auth        *auth.Auth

	Campaigns *common.Campaigns
}

// TODO: fix major bug of closing db on exit
func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	budgetDb := misc.OpenDB(cfg.DBPath, cfg.BudgetDBName)
	reportingDb := misc.OpenDB(cfg.DBPath, cfg.ReportingDBName)

	srv := &Server{
		Cfg:         cfg,
		r:           r,
		db:          db,
		budgetDb:    budgetDb,
		reportingDb: reportingDb,
		auth:        auth.New(db, cfg),
		Campaigns:   common.NewCampaigns(),
	}

	err := srv.initializeDBs(cfg)
	if err != nil {
		return nil, err
	}

	go srv.auth.PurgeInvalidTokens()

	if err = srv.startEngine(); err != nil {
		return nil, err
	}

	srv.initializeRoutes(r)

	return srv, nil
}

func (srv *Server) initializeDBs(cfg *config.Config) error {
	if err := srv.db.Update(func(tx *bolt.Tx) error {
		for _, val := range cfg.AllBuckets() {
			log.Println("Initializing bucket", val)
			if _, err := tx.CreateBucketIfNotExists([]byte(val)); err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			if err := misc.InitIndex(tx, val, 1); err != nil {
				return err
			}
		}

		var u *auth.User
		if u = srv.auth.GetUserTx(tx, auth.AdminUserId); u == nil {
			u = &auth.User{
				Name:  "Sexy Sway Admin",
				Email: adminEmail,
				Type:  auth.AdminScope,
			}
			if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
				return err
			}
			log.Println("created admin user, id = ", u.Id)
		}

		if srv.auth.GetTalentAgencyTx(tx, auth.SwayOpsAgencyId) == nil {
			ag := &auth.TalentAgency{
				Name:   "Sway Ops Talent Agency",
				UserId: u.Id,
				Fee:    0.2,
			}
			if err := srv.auth.CreateTalentAgencyTx(tx, u, ag); err != nil {
				return err
			}
			log.Println("created talent agency, id = ", u.Id)
		}

		if srv.auth.GetAdAgencyTx(tx, auth.SwayOpsAgencyId) == nil {
			ag := &auth.AdAgency{
				Name:   "Sway Ops Ad Agency",
				UserId: u.Id,
				Fee:    0.2,
			}
			if err := srv.auth.CreateAdAgencyTx(tx, u, ag); err != nil {
				return err
			}
			log.Println("created ad agency, id = ", u.Id)
		}

		return nil
	}); err != nil {
		return err
	}

	if err := srv.budgetDb.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(cfg.BudgetBucket)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if err := misc.InitIndex(tx, cfg.BudgetBucket, 1); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	if err := srv.reportingDb.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(cfg.ReportingBucket)); err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if err := misc.InitIndex(tx, cfg.ReportingBucket, 1); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

//TODO should this be in the config?
var scopes = map[string]auth.ScopeMap{
	"talentAgency": {auth.TalentAgencyScope: {Get: true, Post: true, Put: true, Delete: true}},
	"inf": {
		auth.TalentAgencyScope: {Get: true, Post: true, Put: true, Delete: true},
		auth.InfluencerScope:   {Get: true, Post: true, Put: true, Delete: true},
	},
	"adAgency": {auth.AdAgencyScope: {Get: true, Post: true, Put: true, Delete: true}},
	"adv": {
		auth.AdAgencyScope:   {Get: true, Post: true, Put: true, Delete: true},
		auth.AdvertiserScope: {Get: true, Post: true, Put: true, Delete: true},
	},
}

func (srv *Server) initializeRoutes(r *gin.Engine) {
	r.GET("/apiKey", srv.auth.VerifyUser(false), srv.auth.APIKeyHandler)
	r.POST("/signIn", srv.auth.SignInHandler)
	r.POST("/signUp", srv.auth.VerifyUser(true), srv.auth.SignUpHandler)

	// Talent Agency
	createRoutes(r, srv, "/talentAgency", scopes["talentAgency"], auth.TalentAgencyItem, getTalentAgency,
		putTalentAgency, putTalentAgency, delTalentAgency)
	r.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))

	// AdAgency
	createRoutes(r, srv, "/adAgency", scopes["adAgency"], auth.AdAgencyItem, getAdAgency, putAdAgency,
		putAdAgency, delAdAgency)
	r.GET("/getAllAdAgencies", getAllAdAgencies(srv))

	// Advertiser
	createRoutes(r, srv, "/advertiser", scopes["adv"], auth.AdvertiserItem, getAdvertiser, putAdvertiser, putAdvertiser, delAdvertiser)
	r.GET("/getAdvertisersByAgency/:id", getAdvertisersByAgency(srv))

	// Campaigns
	createRoutes(r, srv, "/campaign", scopes["adv"], auth.CampaignItem, getCampaign, postCampaign, putCampaign, delCampaign)

	sh := srv.auth.CheckScopes(scopes["adv"])
	oh := srv.auth.CheckOwnership(auth.CampaignItem, "campaignId")
	r.GET("/getCampaignsByAdvertiser/:id", srv.auth.VerifyUser(false), sh, getCampaignsByAdvertiser(srv))
	r.GET("/getCampaignAssignedDeals/:campaignId", srv.auth.VerifyUser(false), sh, oh, getCampaignAssignedDeals(srv))
	r.GET("/getCampaignCompletedDeals/:campaignId", srv.auth.VerifyUser(false), sh, oh, getCampaignCompletedDeals(srv))

	// Influencers
	createRoutes(r, srv, "/influencer", scopes["inf"], auth.InfluencerItem, getInfluencer, postInfluencer, putInfluencer, delInfluencer)
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

	// Budget
	r.GET("/getBudgetInfo/:id", getBudgetInfo(srv))
	r.GET("/getLastMonthsStore", getLastMonthsStore(srv))
	r.GET("/getStore", getStore(srv))

	// Reporting
	r.GET("/getStats/:cid", getStats(srv))
	r.GET("/getCampaignReport/:cid/:from/:to/:filename", getCampaignReport(srv))

}

func (srv *Server) startEngine() error {
	return newSwayEngine(srv)
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
