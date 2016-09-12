package server

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

const (
	AdminEmail       = "admin@swayops.com"
	AdAdminEmail     = "adAgency@swayops.com"
	TalentAdminEmail = "talentAgency@swayops.com"
	adminPass        = "Rf_j@Z9hM3-"
)

var ErrUserId = errors.New("Unexpected user id")

// Server is the main server of the sway server
type Server struct {
	Cfg      *config.Config
	r        *gin.Engine
	db       *bolt.DB
	budgetDb *bolt.DB
	auth     *auth.Auth

	Campaigns *common.Campaigns
}

// New returns a new Server or an error
// TODO: fix major bug of closing db on exit
func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	initializeDirs(cfg)

	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	budgetDb := misc.OpenDB(cfg.DBPath, cfg.BudgetDBName)

	srv := &Server{
		Cfg:       cfg,
		r:         r,
		db:        db,
		budgetDb:  budgetDb,
		auth:      auth.New(db, cfg),
		Campaigns: common.NewCampaigns(),
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

func initializeDirs(cfg *config.Config) {
	os.MkdirAll(cfg.LogsPath, 0700)
	os.MkdirAll(cfg.LogsPath+"invoices", 0700)
	os.MkdirAll(cfg.DBPath, 0700)
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
			Email: AdminEmail,
			Admin: true,
		}
		if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
			return err
		}
		log.Println("created admin user, id = ", u.ID)

		u = &auth.User{
			ParentID: "1",
			Name:     "Sway Advertiser Agency",
			Email:    AdAdminEmail,
			AdAgency: &auth.AdAgency{},
		}
		if err := srv.auth.CreateUserTx(tx, u, adminPass); err != nil {
			return err
		}

		if u.ID != "2" {
			// Sway advertiser agency must be 2! (for billing)
			return ErrUserId
		}

		log.Println("created advertiser agency, id = ", u.ID)

		u = &auth.User{
			ParentID: "1",
			Name:     "Sway Talent Agency",
			Email:    TalentAdminEmail,
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
	// Public endpoint
	r.GET("/click/:influencerId/:campaignId/:dealId", click(srv))

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
	// NOTE: Check with Ahmed and make this so that talent agencies can view this shit
	verifyGroup.GET("/getInfluencersByAgency/:id", getInfluencersByAgency(srv))
	verifyGroup.GET("/getAgencyInfluencerStats/:id/:infId/:days", getAgencyInfluencerStats(srv))

	adminGroup.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))
	adminGroup.POST("/setBan/:influencerId/:state", setBan(srv))
	adminGroup.GET("/getAllActiveDeals", getAllActiveDeals(srv))

	// AdAgency
	createRoutes(verifyGroup, srv, "/adAgency", "id", scopes["adAgency"], auth.AdAgencyItem, getAdAgency, nil,
		putAdAgency, nil)

	adminGroup.GET("/getAllAdAgencies", getAllAdAgencies(srv))

	// Advertiser
	createRoutes(verifyGroup, srv, "/advertiser", "id", scopes["adv"], auth.AdvertiserItem, getAdvertiser, nil,
		putAdvertiser, nil)
	verifyGroup.GET("/getAdvertiserContentFeed/:id", getAdvertiserContentFeed(srv))
	verifyGroup.GET("/advertiserBan/:id/:influencerId", advertiserBan(srv))

	createRoutes(verifyGroup, srv, "/getAdvertisersByAgency", "id", scopes["adAgency"], auth.AdAgencyItem,
		getAdvertisersByAgency, nil, nil, nil)

	// Campaigns
	createRoutes(verifyGroup, srv, "/campaign", "id", scopes["adv"], auth.CampaignItem, getCampaign, postCampaign,
		putCampaign, delCampaign)

	createRoutes(verifyGroup, srv, "/getCampaignsByAdvertiser", "id", scopes["adv"], auth.AdAgencyItem,
		getCampaignsByAdvertiser, nil, nil, nil)
	verifyGroup.POST("/uploadImage/:id/:bucket", uploadImage(srv))
	r.Static("images", srv.Cfg.ImagesDir)

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
		nil, nil, nil)

	adminGroup.GET("/getInfluencersByCategory/:category", getInfluencersByCategory(srv))
	verifyGroup.GET("/setPlatform/:influencerId/:platform/:id", infOwnership, setPlatform(srv))
	verifyGroup.GET("/setCategory/:influencerId/:category", infOwnership, setCategory(srv))
	verifyGroup.GET("/getCategories", getCategories(srv))
	verifyGroup.GET("/setInviteCode/:influencerId/:inviteCode", infOwnership, infScope, infOwnership, setInviteCode(srv))
	verifyGroup.POST("/setGender/:influencerId/:gender", infOwnership, setGender(srv))
	verifyGroup.POST("/setReminder/:influencerId/:state", infOwnership, setReminder(srv))
	verifyGroup.POST("/setAddress/:influencerId", infOwnership, setAddress(srv))
	verifyGroup.GET("/requestCheck/:influencerId", infScope, infOwnership, requestCheck(srv))
	verifyGroup.GET("/getLatestGeo/:influencerId", infOwnership, getLatestGeo(srv))

	// Budget
	adminGroup.GET("/getBudgetInfo/:id", getBudgetInfo(srv))
	adminGroup.GET("/getLastMonthsStore", getLastMonthsStore(srv))
	adminGroup.GET("/getStore", getStore(srv))

	// Reporting
	advScope := srv.auth.CheckScopes(scopes["adv"])
	campOwnership := srv.auth.CheckOwnership(auth.CampaignItem, "cid")
	verifyGroup.GET("/getCampaignReport/:cid/:from/:to/:filename", advScope, campOwnership, getCampaignReport(srv))
	verifyGroup.GET("/getCampaignStats/:cid/:days", advScope, campOwnership, getCampaignStats(srv))
	verifyGroup.GET("/getCampaignInfluencerStats/:cid/:infId/:days", advScope, campOwnership, getCampaignInfluencerStats(srv))
	verifyGroup.GET("/getInfluencerStats/:influencerId/:days", getInfluencerStats(srv))

	adminGroup.GET("/billing", runBilling(srv))
	adminGroup.GET("/getPendingChecks", getPendingChecks(srv))
	adminGroup.GET("/approveCheck/:influencerId", approveCheck(srv))

	// Get influencers who haven't had biodata filled by admin
	adminGroup.GET("/getIncompleteInfluencers", getIncompleteInfluencers(srv))

	// Perks
	adminGroup.GET("/getPendingCampaigns", getPendingCampaigns(srv))
	adminGroup.GET("/approveCampaign/:id", approveCampaign(srv))
	adminGroup.GET("/getPendingPerks", getPendingPerks(srv))
	adminGroup.GET("/approvePerk/:influencerId/:campaignId", approvePerk(srv))

	adminGroup.GET("/forceApprove/:influencerId/:campaignId", forceApproveAny(srv))
	adminGroup.GET("/forceDeplete", forceDeplete(srv))

	adminGroup.GET("/emailTaxForm/:influencerId", emailTaxForm(srv))

	// Scraps
	adminGroup.GET("/scrap/:id", getScrap(srv))
	adminGroup.POST("/scrap", postScrap(srv))
	adminGroup.PUT("/scrap/:id", putScrap(srv))
	adminGroup.GET("/getIncompleteScraps", getIncompleteScraps(srv))
	// Run emailing of deals right now
	adminGroup.GET("/forceEmail", forceEmail(srv))
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
