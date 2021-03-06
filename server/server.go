package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/hoisie/mustache"
	"github.com/stripe/stripe-go"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

const (
	AdminEmail       = "admin@swayops.com"
	AdAdminEmail     = "adAgency@swayops.com"
	TalentAdminEmail = "talentAgency@swayops.com"
	adminPass        = "Rf_j@Z9hM3-"
)

var (
	gitBuild    string = "n/a"
	ErrUserId          = errors.New("Unexpected user id")
	mailingList        = []string{"shahzil@swayops.com", "nick@swayops.com"}
)

// Server is the main server of the sway server
type Server struct {
	Cfg  *config.Config
	r    *gin.Engine
	db   *bolt.DB
	auth *auth.Auth

	Campaigns *common.Campaigns
	Audiences *common.Audiences
	Scraps    *influencer.Scraps
	Forecasts *Forecasts

	Categories []*InfCategory // List of available categories and their reach

	LimitSet *common.LimitSet

	ClickSet *common.Set

	Stats ServerStats // stores most recent server (engine) stats
}

type ServerStats struct {
	mux                sync.RWMutex
	LastRun            int64 `json:"lastRun,omitempty"`    // Last engine run time
	Bootup             int64 `json:"bootup,omitempty"`     // Time the server was booted up
	InfluencersUpdated int32 `json:"infUpdated,omitempty"` // Influencers updated in the last engine run
}

func NewStats() ServerStats {
	return ServerStats{
		Bootup: time.Now().Unix(),
	}
}

func (ss *ServerStats) Get() (stats *ServerStats) {
	ss.mux.RLock()
	stats = &ServerStats{
		LastRun:            ss.LastRun,
		InfluencersUpdated: ss.InfluencersUpdated,
		Bootup:             ss.Bootup,
	}
	ss.mux.RUnlock()
	return
}

func (ss *ServerStats) Update(updated int32, lastRun int64) {
	ss.mux.Lock()
	ss.LastRun = lastRun
	ss.InfluencersUpdated = updated
	ss.mux.Unlock()
}

// New returns a new Server or an error
// TODO: fix major bug of closing db on exit
func New(cfg *config.Config, r *gin.Engine) (*Server, error) {
	initializeDirs(cfg)

	db := misc.OpenDB(cfg.DBPath, cfg.DBName)

	srv := &Server{
		Cfg:       cfg,
		r:         r,
		db:        db,
		auth:      auth.New(db, cfg),
		Campaigns: common.NewCampaigns(nil),
		Audiences: common.NewAudiences(),
		LimitSet:  common.NewLimitSet(),
		ClickSet:  common.NewSet(),
		Forecasts: NewForecasts(),
		Scraps:    influencer.NewScraps(),
		Stats:     NewStats(),
	}

	stripe.Key = cfg.Stripe.Key
	if cfg.Sandbox {
		stripe.LogLevel = 0
	}

	err := srv.initializeDBs(cfg)
	if err != nil {
		return nil, err
	}

	go srv.auth.PurgeInvalidTokens()

	if err = srv.startEngine(); err != nil {
		return nil, err
	}

	srv.Categories = getAllCategories(srv)

	srv.initializeRoutes(r)

	return srv, nil
}

func initializeDirs(cfg *config.Config) {
	os.MkdirAll(cfg.LogsPath, 0700)
	os.MkdirAll(filepath.Join(cfg.LogsPath, "invoices"), 0700)
	os.MkdirAll(cfg.DBPath, 0700)
	os.MkdirAll(filepath.Join(cfg.ImagesDir, "deal"), 0700)
}

func (srv *Server) initializeDBs(cfg *config.Config) error {
	if err := srv.db.Update(func(tx *bolt.Tx) error {
		for _, val := range cfg.AllBuckets(cfg.Bucket) {
			log.Println("Initializing bucket", val)
			if _, err := tx.CreateBucketIfNotExists([]byte(val)); err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			idxStart := uint64(1)
			if val == cfg.Bucket.URL {
				idxStart = 10000
			}
			if err := misc.InitIndex(tx, val, idxStart); err != nil {
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

		if cfg.Sandbox {
			u.AdAgency.IsIO = true
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

// TODO: clean this up or move to meteora router
func getDashRoutes(srv *Server) func(c *gin.Context) {
	var (
		idxFile     = filepath.Join(srv.Cfg.DashboardPath, "index.html")
		favIcoFile  = filepath.Join(srv.Cfg.DashboardPath, "/static/img/favicon.ico")
		staticGzer  = staticGzipServe(filepath.Join(srv.Cfg.DashboardPath, "static"))
		idxFileHTML []byte
	)
	tmpl, err := mustache.ParseFile(idxFile)
	if err != nil {
		log.Panic(err)
	}
	idxFileHTML = []byte(tmpl.Render(gin.H{"infAppUrl": srv.Cfg.InfAppURL}))

	return func(c *gin.Context) {
		p := c.Request.URL.Path[1:]
		parts := strings.Split(p, "/")
		if len(parts) > 0 {
			p = parts[0]
		}

		switch p {
		case "api":
			return
		case "invite":
			if len(parts) == 2 {
				c.Redirect(308, srv.Cfg.InfAppURL+"/signup/"+parts[1])
			} else {
				misc.AbortWithErr(c, 400, auth.ErrInvalidRequest)
			}
			return
		case "favicon.ico":
			c.File(favIcoFile)
		case "static":
			staticGzer(c)
			return
		case "views":
			c.File(filepath.Join(srv.Cfg.DashboardPath, "app", "views", parts[1]))
		default:
			c.Data(200, gin.MIMEHTML, idxFileHTML)
		}
		c.Abort()
	}
}

func getInfAppRoutes(srv *Server) func(c *gin.Context) {
	var (
		idxFile    = filepath.Join(srv.Cfg.InfAppPath, "index.html")
		favIcoFile = filepath.Join(srv.Cfg.InfAppPath, "/static/img/favicon.ico")
		staticGzer = staticGzipServe(filepath.Join(srv.Cfg.InfAppPath, "static"))
	)
	return func(c *gin.Context) {
		p := c.Request.URL.Path[1:]
		parts := strings.Split(p, "/")
		if len(parts) > 0 {
			p = parts[0]
		}
		serve := idxFile
		switch p {
		case "api":
			return
		case "favicon.ico":
			serve = favIcoFile
		case "views":
			c.File(filepath.Join(srv.Cfg.InfAppPath, "app", "views", parts[1]))
		case "static":
			staticGzer(c)
			return
		}
		c.File(serve)
		c.Abort()
	}
}

func (srv *Server) initializeRoutes(r gin.IRouter) {
	staticGzer := staticGzipServe("./images/")
	r.HEAD("/images/*fp", staticGzer)
	r.GET("/images/*fp", staticGzer)

	infAppRoutes := getInfAppRoutes(srv)
	dashRoutes := getDashRoutes(srv)

	r.Use(func(c *gin.Context) {
		var subdomain string
		if dot := strings.IndexRune(c.Request.Host, '.'); dot > -1 {
			subdomain = c.Request.Host[:dot]
		}
		switch subdomain {
		case "":
		case "inf":
			infAppRoutes(c)
		case "dash":
			dashRoutes(c)
		}
	})

	r = r.Group(srv.Cfg.APIPath)

	r.GET("/version", func(c *gin.Context) {
		misc.WriteJSON(c, 200, gin.H{"version": gitBuild})
	})

	// Public endpoint
	r.GET("/cl/*id", click(srv))
	r.GET("/c/*id", click(srv))

	// Test endpoint.. delete later
	r.GET("/inf/test/:id", testInfluencer(srv))
	r.POST("/search", getForecast(srv, true))
	r.GET("/forecast/user/:id", getForecastUser(srv))

	r.GET("/optout/:email", optoutScrap(srv))
	r.GET("/value/:platform/:handle", influencerValue(srv))

	// Key based auth
	r.GET("/getContentFeed/:id", getAdvertiserContentFeed(srv, true))

	verifyGroup := r.Group("", srv.auth.VerifyUser(false))
	adminGroup := verifyGroup.Group("", srv.auth.CheckScopes(nil))

	// /apiKey easier takes the GET request of a logged in user or
	// POST with the user's email/password
	verifyGroup.GET("/apiKey", srv.auth.APIKeyHandler)
	r.POST("/apiKey", srv.auth.APIKeyHandler)

	verifyGroup.GET("/signOut", srv.auth.SignOutHandler)
	r.POST("/signIn", srv.auth.SignInHandler)
	r.POST("/signUp", srv.auth.VerifyUser(true), srv.auth.SignUpHandler)

	r.POST("/forgotPassword", srv.auth.ReqResetHandler)
	r.POST("/resetPassword", srv.auth.ResetHandler)

	verifyGroup.GET("/user", userProfile(srv))

	verifyGroup.GET("/user/:id", userProfile(srv))

	adminGroup.PUT("/admin/:id", putAdmin(srv))                   // save profile for admin
	adminGroup.PUT("/hack/changeLogin/:id", changeLogin(srv))     // changeLogin
	adminGroup.GET("/hack/removeSubUser/:id", removeSubUser(srv)) // removeSubUser

	// Talent Agency
	createRoutes(verifyGroup, srv, "/talentAgency", "id", scopes["talentAgency"], auth.TalentAgencyItem, getTalentAgency,
		nil, putTalentAgency, nil)
	// NOTE: Check with Ahmed and make this so that talent agencies can view this shit
	verifyGroup.GET("/getInfluencersByAgency/:id", getInfluencersByAgency(srv))
	verifyGroup.GET("/getAgencyInfluencerStats/:id/:infId/:days", getAgencyInfluencerStats(srv))

	adminGroup.GET("/getAllTalentAgencies", getAllTalentAgencies(srv))
	adminGroup.POST("/setBan/:influencerId/:state", setBan(srv))
	adminGroup.GET("/getAllActiveDeals", getAllActiveDeals(srv))
	adminGroup.GET("/setKeyword/:influencerId/:kw", addKeyword(srv))
	adminGroup.GET("/addDeals/:campaignId/:count", addDealCount(srv))
	// adminGroup.GET("/setSignature/:influencerId/:sigId", setSignature(srv))
	adminGroup.POST("/addBonus", addBonus(srv))
	adminGroup.GET("/skipGeo/:influencerId/:campaignId", skipGeo(srv))
	adminGroup.GET("/skipYield/:influencerId/:campaignId", skipYield(srv))

	adminGroup.GET("/getAllHandles/:platform", getAllHandles(srv))

	adminGroup.POST("/setScrap", setScrap(srv))
	adminGroup.GET("/getScrap/:id", getScrap(srv))
	adminGroup.GET("/getScrapByHandle/:id/:platform", getScrapByHandle(srv))
	adminGroup.GET("/getScraps", getScraps(srv))
	adminGroup.GET("/getScrapStats", scrapStats(srv))
	adminGroup.GET("/unapproveDeal/:influencerId/:dealId", unapproveDeal(srv))

	adminGroup.GET("/dumpDatabases", dumpDatabases(srv))

	adminGroup.GET("/setBan/:influencerId/:state", setBan(srv))
	adminGroup.GET("/setFraud/:campaignId/:influencerId/:state", setFraud(srv))
	adminGroup.GET("/setStrike/:campaignId/:influencerId/:reasons", setStrike(srv))

	// AdAgency
	createRoutes(verifyGroup, srv, "/adAgency", "id", scopes["adAgency"], auth.AdAgencyItem, getAdAgency, nil,
		putAdAgency, nil)

	adminGroup.GET("/getAllAdAgencies", getAllAdAgencies(srv))

	// Advertiser
	createRoutes(verifyGroup, srv, "/advertiser", "id", scopes["adv"], auth.AdvertiserItem, getAdvertiser, nil,
		putAdvertiser, nil)

	advScopes := srv.auth.CheckScopes(scopes["adv"])
	verifyGroup.GET("/subUsers/:id", advScopes, srv.auth.ListSubUsersHandler)
	verifyGroup.POST("/subUsers/:id", advScopes, srv.auth.AddSubUserHandler)
	verifyGroup.DELETE("/subUsers/:id/:email", srv.auth.DelSubUserHandler)

	verifyGroup.GET("/getAdvertiserContentFeed/:id", getAdvertiserContentFeed(srv, false))
	verifyGroup.GET("/advertiserBan/:id/:influencerId", advertiserBan(srv))
	verifyGroup.GET("/billingInfo/:id", getBillingInfo(srv))
	verifyGroup.GET("/getAdvertiserTimeline/:id", getAdvertiserTimeline(srv))
	verifyGroup.GET("/approveSubmission/:id/:campaignId/:influencerId", approveSubmission(srv))

	adminGroup.GET("/balance/:id", getBalance(srv))
	adminGroup.GET("/getCampaignStore", getCampaignStore(srv))

	createRoutes(verifyGroup, srv, "/getAdvertisersByAgency", "id", scopes["adAgency"], auth.AdAgencyItem,
		getAdvertisersByAgency, nil, nil, nil)

	// Campaigns
	createRoutes(verifyGroup, srv, "/campaign", "id", scopes["adv"], auth.CampaignItem, getCampaign, postCampaign,
		putCampaign, delCampaign)

	createRoutes(verifyGroup, srv, "/getCampaignsByAdvertiser", "id", scopes["adv"], auth.AdAgencyItem,
		getCampaignsByAdvertiser, nil, nil, nil)
	verifyGroup.POST("/uploadImage/:id/:bucket", uploadImage(srv))
	verifyGroup.GET("/getDealsForCampaign/:id", getDealsForCampaign(srv))
	verifyGroup.GET("/getTargetYield/:id", getTargetYield(srv))
	verifyGroup.POST("/getForecast", getForecast(srv, false))
	verifyGroup.POST("/getForecastExport/:filename", getForecastExport(srv))
	verifyGroup.GET("/inventory/:state", getInventoryByState(srv))
	verifyGroup.GET("/getMatchesForKeyword/:kw", getMatchesForKeyword(srv))
	verifyGroup.GET("/getKeywords", getKeywords(srv))
	verifyGroup.GET("/unassignDeal/:influencerId/:campaignId/:dealId", unassignDeal(srv))
	verifyGroup.GET("/dirtyHack", dirtyHack(srv))
	verifyGroup.GET("/getRejections/:campaignId/:influencerId", getRejections(srv))
	verifyGroup.GET("/getCycle/:id", getCycle(srv))

	r.Static("images", srv.Cfg.ImagesDir)

	// Deal
	infScope := srv.auth.CheckScopes(scopes["inf"])
	infOwnership := srv.auth.CheckOwnership(auth.InfluencerItem, "influencerId")
	verifyGroup.GET("/getDeals/:influencerId/:lat/:long", infScope, infOwnership, getDealsForInfluencer(srv))
	verifyGroup.GET("/getDeal/:influencerId/:campaignId/:dealId", infScope, infOwnership, getDeal(srv))
	verifyGroup.GET("/assignDeal/:influencerId/:campaignId/:dealId/:platform", infScope, infOwnership, assignDeal(srv))
	verifyGroup.GET("/getDealsAssigned/:influencerId", infScope, infOwnership, getDealsAssignedToInfluencer(srv))
	verifyGroup.GET("/getDealsCompleted/:influencerId", infScope, infOwnership, getDealsCompletedByInfluencer(srv))
	verifyGroup.GET("/getCompletedDeal/:influencerId/:dealId", infOwnership, getCompletedDeal(srv))
	// verifyGroup.GET("/emailTaxForm/:influencerId", infScope, emailTaxForm(srv))
	verifyGroup.GET("/sendInstructions/:influencerId/:campaignId/:dealId", infScope, infOwnership, sendInstructions(srv))
	verifyGroup.POST("/submitPost/:influencerId/:campaignId", infScope, submitPost(srv))

	// Influencers
	createRoutes(verifyGroup, srv, "/influencer", "id", scopes["inf"], auth.InfluencerItem, getInfluencer,
		nil, putInfluencer, nil)

	adminGroup.GET("/getInfluencersByCategory/:category", getInfluencersByCategory(srv))
	adminGroup.PUT("/setAudit/:influencerId", setAudit(srv))
	adminGroup.GET("/setAgency/:influencerId/:agencyId", setAgency(srv))
	verifyGroup.GET("/getCategories", getCategories(srv))
	verifyGroup.GET("/requestCheck/:influencerId", infScope, infOwnership, requestCheck(srv))
	verifyGroup.GET("/getLatestGeo/:influencerId", infOwnership, getLatestGeo(srv))
	verifyGroup.GET("/bio/:influencerId", infOwnership, getBio(srv))
	verifyGroup.GET("/blockCampaign/:influencerId/:campaignId", infOwnership, blockCampaign(srv))

	// Budget
	adminGroup.GET("/getBudgetInfo/:id", getBudgetInfo(srv))
	adminGroup.GET("/getStore", getStore(srv))
	adminGroup.GET("/getBudgetSnapshot", getBudgetSnapshot(srv))

	// Reporting
	advScope := srv.auth.CheckScopes(scopes["adv"])
	campOwnership := srv.auth.CheckOwnership(auth.CampaignItem, "cid")
	verifyGroup.GET("/getAdvertiserStats/:id/:start/:end", getAdvertiserStats(srv))
	verifyGroup.GET("/getCampaignReport/:cid/:from/:to/:filename", advScope, campOwnership, getCampaignReport(srv))
	verifyGroup.GET("/getCampaignStats/:cid/:days", advScope, campOwnership, getCampaignStats(srv))
	verifyGroup.GET("/getCampaignInfluencerStats/:cid/:infId/:days", advScope, campOwnership, getCampaignInfluencerStats(srv))
	verifyGroup.GET("/getInfluencerStats/:influencerId/:days", getInfluencerStats(srv))
	adminGroup.GET("/getAdminStats", getAdminStats(srv))

	adminGroup.GET("/forceBill/:id", forceBill(srv))
	adminGroup.GET("/forceDeduction/:id/:amount", forceDeduction(srv))
	adminGroup.GET("/forceRefund", forceRefund(srv))

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
	adminGroup.POST("/forceApprovePost", forceApprovePost(srv))
	adminGroup.GET("/forceDeplete", forceDeplete(srv))
	adminGroup.GET("/forceEngine", forceEngine(srv))
	adminGroup.GET("/forceScrapEmail", forceScrapEmail(srv))
	adminGroup.GET("/forceAttributer", forceAttributer(srv))
	adminGroup.GET("/forceTimeline", forceTimeline(srv))
	adminGroup.GET("/syncHack", syncAllStats(srv))
	adminGroup.GET("/assignLikelyEarnings", assignLikelyEarnings(srv))
	adminGroup.GET("/getTotalClicks/:hours", getTotalClicks(srv))
	adminGroup.GET("/exportClicks/:days", exportClicks(srv))
	adminGroup.GET("/serverStats", getServerStats(srv))
	adminGroup.GET("/emptyPayout/:influencerId", emptyPayout(srv))

	// Run emailing of deals right now
	adminGroup.GET("/forceEmail", forceEmail(srv))

	// Audiences
	// Agency audiences
	agencyScopes := srv.auth.CheckScopes(scopes["adAgency"])
	// POST method for agency audience
	verifyGroup.POST("/agency/audience/:id", agencyScopes, agencyAudience(srv))
	verifyGroup.GET("/getAgencyAudience/:audID", agencyScopes, getAudience(srv))
	verifyGroup.DELETE("/agency/audience/:id/:audID", agencyScopes, delAgencyAudience(srv))

	// Advertiser audiences
	// POST method for advertiser audience
	verifyGroup.POST("/advertiser/audience/:id", advScopes, advertiserAudience(srv))
	verifyGroup.GET("/getAdvertiserAudience/:audID", advScopes, getAudience(srv))
	verifyGroup.DELETE("/advertiser/audience/:id/:audID", advScopes, delAdvertiserAudience(srv))

	// GET all relevant audiences for the user
	verifyGroup.GET("/getUserAudiences/:id", advScopes, getUserAudiences(srv))

	// Admin audiences
	adminGroup.POST("/audience", adminAudience(srv))
	adminGroup.DELETE("/audience/:id", delAudience(srv))
	adminGroup.GET("/audience", getAudiences(srv))
	adminGroup.GET("/audience/:id", getAudiences(srv))
}

func (srv *Server) startEngine() error {
	return newSwayEngine(srv)
}

func redirectToHTTPS(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "https://"+req.Host+req.URL.String(), http.StatusMovedPermanently)
}

// Run starts the server
func (srv *Server) Run() error {
	var (
		errCh   = make(chan error, 2)
		host    = srv.Cfg.Host
		sandbox = srv.Cfg.Sandbox
	)
	if host == "" {
		host = "*.swayops.com"
	}
	go func() {
		if sandbox {
			log.Printf("listening on http://%s:%s", host, srv.Cfg.Port)
			errCh <- srv.r.Run(srv.Cfg.Host + ":" + srv.Cfg.Port)
		} else {
			log.Printf("listening on http://%s:%s and redirecting to https", host, srv.Cfg.Port)
			errCh <- http.ListenAndServe(srv.Cfg.Host+":"+srv.Cfg.Port, http.HandlerFunc(redirectToHTTPS))
		}
	}()
	if tls := srv.Cfg.TLS; tls != nil {
		go func() {
			log.Printf("listening on https://%s:%s", host, tls.Port)
			errCh <- srv.r.RunTLS(srv.Cfg.Host+":"+tls.Port, tls.Cert, tls.Key)
		}()
	}
	return <-errCh
}

func (srv *Server) Alert(msg string, err error) {
	if srv.Cfg.Sandbox {
		return
	}

	log.Println(msg, err)

	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	email := templates.ErrorEmail.Render(map[string]interface{}{"error": errMsg, "msg": msg})
	for _, addr := range mailingList {
		if resp, err := srv.Cfg.MailClient().SendMessage(email, "Critical error!", addr, "Important Person",
			[]string{}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			log.Println("Error sending alert email!")
		}
	}
}

func (srv *Server) Notify(subject, msg string) {
	if srv.Cfg.Sandbox {
		return
	}

	email := templates.NotifyEmail.Render(map[string]interface{}{"msg": msg})

	for _, addr := range mailingList {
		if resp, err := srv.Cfg.MailClient().SendMessage(email, subject, addr, "Important Person",
			[]string{}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			log.Println("Error sending notify email!")
		}
	}
}

func (srv *Server) Fraud(cid, infId, url string, reasons []string) {
	if srv.Cfg.Sandbox {
		return
	}

	allowURL := fmt.Sprintf("setFraud/%s/%s/true", cid, infId)
	strikeURL := fmt.Sprintf("setStrike/%s/%s/%s", cid, infId, strings.Join(reasons, ","))
	banURL := fmt.Sprintf("setBan/%s/true", infId)

	email := templates.FraudEmail.Render(map[string]interface{}{
		"CampaignID":   cid,
		"InfluencerID": infId,
		"URL":          url,
		"Reasons":      reasons,
		"AllowURL":     allowURL,
		"StrikeURL":    strikeURL,
		"BanURL":       banURL,
	})

	for _, addr := range mailingList {
		if resp, err := srv.Cfg.MailClient().SendMessage(email, fmt.Sprintf("Approval required for campaign %s and influencer id %s", cid, infId), addr, "Important Person",
			[]string{}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			log.Println("Error sending fraud email!")
		}
	}
}

func (srv *Server) Digest(updatedInf, foundDeals int32, depletions []*Depleted, dealsEmailed, scrapsEmailed int32, start time.Time) {
	if srv.Cfg.Sandbox {
		return
	}

	now := time.Now()

	var totalSpent float64
	for _, d := range depletions {
		totalSpent += d.Spent
	}

	if foundDeals+dealsEmailed+scrapsEmailed == 0 && totalSpent < 1 {
		return
	}

	load := map[string]interface{}{
		"startTime":     start.String(),
		"updatedInf":    updatedInf,
		"foundDeals":    foundDeals,
		"totalSpent":    misc.TruncateFloat(totalSpent, 2),
		"depletions":    depletions,
		"sigsFound":     0,
		"dealsEmailed":  dealsEmailed,
		"scrapsEmailed": scrapsEmailed,
		"endTime":       now.String(),
		"runtime":       now.Unix() - start.Unix(),
	}

	email := templates.EngineEmail.Render(load)
	for _, addr := range mailingList {
		if resp, err := srv.Cfg.MailClient().SendMessage(email, "Engine Digest", addr, "Important Person",
			[]string{}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
			log.Println("Error sending engine digest email email!")
		}
	}
}

func (srv *Server) Close() error {
	log.Println("exiting...")

	// srv.r.Close() // not implemented in gin nor net/http
	srv.db.Close()
	srv.Cfg.Loggers.Close()

	return nil
}
