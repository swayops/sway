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

const (
	adminEmail  = "admin@swayops.com"
	adminPass   = "Rf_jv9hM3-"
	agencyEmail = "agency@swayops.com"
	agencyPass  = "Rf_jv9hM4-"
)

type Server struct {
	Cfg      *config.Config
	r        *gin.Engine
	db       *bolt.DB
	budgetDb *bolt.DB
	authDb   *bolt.DB
	auth     *auth.Auth
}

// TODO: fix major bug of closing db on exit
func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	budgetDb := misc.OpenDB(cfg.DBPath, cfg.BudgetDBName)
	authDb := misc.OpenDB(cfg.DBPath, cfg.AuthDBName)

	srv := &Server{
		Cfg:      cfg,
		r:        r,
		db:       db,
		budgetDb: budgetDb,
		authDb:   authDb,
		auth:     auth.New(authDb, cfg),
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

	return srv.authDb.Update(func(tx *bolt.Tx) error {
		for _, val := range cfg.AllAuthBuckets() {
			log.Println("Initializing bucket", val)
			if _, err := tx.CreateBucketIfNotExists([]byte(val)); err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			if err := misc.InitIndex(tx, val, 1); err != nil {
				return err
			}
		}
		if srv.auth.GetUserTx(tx, auth.AdminUserId) == nil {
			u := auth.User{
				Name:  "System Admin",
				Email: adminEmail,
				Type:  auth.Admin,
			}
			if err := srv.auth.CreateUserTx(tx, &u, adminPass); err != nil {
				return err
			}
			log.Println("created admin user, id = ", u.Id)
		}
		// is this needed?
		if srv.auth.GetUserTx(tx, auth.SwayOpsAgencyId) == nil {
			u := auth.User{
				Name:     "SwayOps Agency",
				Email:    agencyEmail,
				Type:     auth.AdvertiserAgency,
				ParentId: auth.AdminUserId,
			}
			if err := srv.auth.CreateUserTx(tx, &u, agencyPass); err != nil {
				return err
			}
			log.Println("created agency, id = ", u.Id)
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

func (srv *Server) initializeRoutes(r *gin.Engine) {
	r.POST("/signin", srv.auth.SignInHandler)
	r.POST("/signup", srv.auth.SignUpHandler)
	// Talent Agency
	createRoutes(r, srv, "/talentAgency", scopes["talentAgency"], auth.TalentAgencyItem, getTalentAgency, putTalentAgency, delTalentAgency)
	r.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))
	r.POST("/updateTalentAgency/:id", updateTalentAgency(srv))

	// AdAgency
	createRoutes(r, srv, "/adAgency", scopes["adAgency"], auth.AdvertiserAgencyItem, getAdAgency, putAdAgency, delAdAgency)
	r.GET("/getAllAdAgencies", getAllAdAgencies(srv))
	r.POST("/updateAdAgency/:id", updateAdAgency(srv))

	// Advertiser
	createRoutes(r, srv, "/advertiser", scopes["adv"], auth.AdvertiserItem, getAdvertiser, putAdvertiser, delAdvertiser)
	r.GET("/getAdvertisersByAgency/:id", getAdvertisersByAgency(srv))
	r.POST("/updateAdvertiser/:id", updateAdvertiser(srv))

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
	// r.GET("/setFloor/:influencerId/:floor", setFloor(srv))

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
