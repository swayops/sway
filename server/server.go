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
	adminEmail       = "admin@swayops.com"
	adAdminEmail     = "adAgency@swayops.com"
	talentAdminEmail = "talentAgency@swayops.com"
	adminPass        = "Rf_jv9hM3-"
)

// Server is the main server of the sway server
type Server struct {
	Cfg         *config.Config
	r           *gin.Engine
	db          *bolt.DB
	budgetDb    *bolt.DB
	reportingDb *bolt.DB
	auth        *auth.Auth

	Campaigns *common.Campaigns
}

// New returns a new Server or an error
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

		if srv.auth.GetUserTx(tx, auth.AdminUserID) != nil {
			return nil
		}
		u := &auth.User{
			Name:  "Sway Admin",
			Email: adminEmail,
			Type:  auth.AdminScope,
		}
		if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
			return err
		}
		log.Println("created admin user, id = ", u.ID)

		u = &auth.User{
			ParentID: "1",
			Name:     "Sway Advertiser Agency",
			Email:    adAdminEmail,
			Type:     auth.AdAgencyScope,
			AdAgency: &auth.AdAgency{},
		}
		if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
			return err
		}
		log.Println("created advertiser agency, id = ", u.ID)

		u = &auth.User{
			ParentID: "1",
			Name:     "Sway Talent Agency",
			Email:    talentAdminEmail,
			Type:     auth.TalentAgencyScope,
			TalentAgency: &auth.TalentAgency{
				Fee: 0.2,
			},
		}
		if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
			return err
		}
		log.Println("created Talent agency, id = ", u.ID)

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
	verifyGroup := r.Group("", srv.auth.VerifyUser(false))
	adminGroup := verifyGroup.Group("", srv.auth.CheckScopes(nil))

	// /apiKey easier takes the GET request of a logged in user or
	// POST with the user's email/password
	verifyGroup.GET("/apiKey", srv.auth.APIKeyHandler)
	r.POST("/apiKey", srv.auth.APIKeyHandler)

	verifyGroup.GET("/signOut", srv.auth.SignOutHandler)
	r.POST("/signIn", srv.auth.SignInHandler)
	r.POST("/signUp", srv.auth.VerifyUser(true), srv.auth.SignUpHandler)

	// Talent Agency
	createRoutes(verifyGroup, srv, "/talentAgency", "id", scopes["talentAgency"], auth.TalentAgencyItem, getTalentAgency,
		nil, putTalentAgency, nil)

	adminGroup.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))

	// AdAgency
	createRoutes(verifyGroup, srv, "/adAgency", "id", scopes["adAgency"], auth.AdAgencyItem, getAdAgency, nil,
		putAdAgency, nil)

	adminGroup.GET("/getAllAdAgencies", getAllAdAgencies(srv))

	// Advertiser
	createRoutes(verifyGroup, srv, "/advertiser", "id", scopes["adv"], auth.AdvertiserItem, getAdvertiser, nil,
		putAdvertiser, nil)

	createRoutes(verifyGroup, srv, "/getAdvertisersByAgency", "id", scopes["adAgency"], auth.AdAgencyItem,
		getAdvertisersByAgency, nil, nil, nil)

	// Campaigns
	createRoutes(verifyGroup, srv, "/campaign", "id", scopes["adv"], auth.CampaignItem, getCampaign, postCampaign,
		putCampaign, delCampaign)

	createRoutes(verifyGroup, srv, "/getCampaignsByAdvertiser", "id", scopes["adv"], auth.AdAgencyItem,
		getCampaignsByAdvertiser, nil, nil, nil)

	// Deal
	infScope := srv.auth.CheckScopes(scopes["inf"])
	infOwnership := srv.auth.CheckOwnership(auth.InfluencerItem, "influencerId")
	verifyGroup.GET("/getDeals/:influencerId/:lat/:long", infScope, infOwnership, getDealsForInfluencer(srv))
	verifyGroup.GET("/assignDeal/:influencerId/:campaignId/:dealId/:platform", infScope, infOwnership, assignDeal(srv))
	verifyGroup.GET("/unassignDeal/:influencerId/:campaignId/:dealId", infScope, infOwnership, unassignDeal(srv))
	verifyGroup.GET("/getDealsAssigned/:influencerId", infScope, infOwnership, getDealsAssignedToInfluencer(srv))
	verifyGroup.GET("/getDealsCompleted/:influencerId", infScope, infOwnership, getDealsCompletedByInfluencer(srv))

	// Influencers
	createRoutes(verifyGroup, srv, "/influencer", "id", scopes["inf"], auth.InfluencerItem, getInfluencer,
		nil, putInfluencer, nil)

	adminGroup.GET("/getInfluencersByCategory/:category", getInfluencersByCategory(srv))
	adminGroup.GET("/getInfluencersByAgency/:agencyId", getInfluencersByAgency(srv))
	verifyGroup.GET("/setPlatform/:influencerId/:platform/:id", infOwnership, setPlatform(srv))
	verifyGroup.GET("/setCategory/:influencerId/:category", infOwnership, setCategory(srv))
	verifyGroup.GET("/getCategories", getCategories(srv))
	verifyGroup.GET("/setInviteCode/:influencerId/:inviteCode", infOwnership, infScope, infOwnership, setInviteCode(srv))

	// Budget
	adminGroup.GET("/getBudgetInfo/:id", getBudgetInfo(srv))
	adminGroup.GET("/getLastMonthsStore", getLastMonthsStore(srv))
	adminGroup.GET("/getStore", getStore(srv))

	// Reporting
	advScope := srv.auth.CheckScopes(scopes["adv"])
	campOwnership := srv.auth.CheckOwnership(auth.CampaignItem, "cid")
	verifyGroup.GET("/getCampaignReport/:cid/:from/:to/:filename", advScope, campOwnership, getCampaignReport(srv))
	verifyGroup.GET("/getCampaignStats/:cid/:days", advScope, campOwnership, getCampaignStats(srv))
	verifyGroup.GET("/getRawStats/:cid", advScope, campOwnership, getRawStats(srv))
	verifyGroup.GET("/getCampaignInfluencerStats/:cid/:infId/:days", advScope, campOwnership, getCampaignInfluencerStats(srv))

	verifyGroup.GET("/getInfluencerStats/:influencerId/:days", getInfluencerStats(srv))
}

func (srv *Server) startEngine() error {
	return newSwayEngine(srv)
}

// Run starts the server
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
