package server

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/swayops/resty"
	// "github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/hellosign"
	"github.com/swayops/sway/platforms/lob"
	"github.com/swayops/sway/platforms/swipe"
)

var (
	adminReq             = M{"email": AdminEmail, "pass": adminPass}
	adminAdAgencyReq     = M{"email": AdAdminEmail, "pass": adminPass}
	adminTalentAgencyReq = M{"email": TalentAdminEmail, "pass": adminPass}
)

func TestAdminLogin(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{
		IsIO: true,
	}

	adv := getSignupUser()
	adv.ParentID = ag.ExpID
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	ag2 := getSignupUser()
	ag2.AdAgency = &auth.AdAgency{
		IsIO: true,
	}

	adv2 := getSignupUser()
	adv2.ParentID = ag.ExpID
	adv2.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"GET", "/apiKey", nil, 200, nil},

		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		// create advertiser as admin and set agency to ag.ExpID
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},

		// create another agency and try to create an advertiser with the old agency's id
		{"POST", "/signUp?autologin=true", ag2, 200, misc.StatusOK(ag2.ExpID)},
		{"POST", "/signUp", adv2, 200, misc.StatusOK(adv2.ExpID)}, // should work but switch the parent id to ag2's
		{"GET", "/advertiser/" + adv2.ExpID, nil, 200, M{
			"agencyId": ag2.ExpID,
		}},
	} {
		tr.Run(t, rst)
	}

}

// this includes child advertiser tests
func TestAdAgencyChain(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{
		IsIO: true,
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	for _, tr := range [...]*resty.TestRequest{
		// try to directly signup as an agency
		{"POST", "/signUp", ag, 401, nil},

		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},
		{"POST", "/signIn", M{"email": ag.Email, "pass": defaultPass}, 200, nil},

		// change the agency's name
		{"PUT", "/adAgency/" + ag.ExpID, &auth.User{AdAgency: &auth.AdAgency{ID: ag.ExpID, Name: "the rain man", Status: true, IsIO: true}}, 200, nil},
		{"GET", "/adAgency/" + ag.ExpID, nil, 200, M{"name": "the rain man", "io": true}},

		// create a new advertiser as the new agency and signin
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		{"POST", "/signIn", M{"email": adv.Email, "pass": defaultPass}, 200, nil},

		// ban a user for this adv
		{"GET", "/advertiserBan/" + adv.ExpID + "/randomInf", nil, 200, nil},

		// update the advertiser and check if the update worked
		{"PUT", "/advertiser/" + adv.ExpID, &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1}}, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: ag.ExpID, DspFee: 0.1, ExchangeFee: 0.2, Blacklist: map[string]bool{"randomInf": true}}},

		// sign in as admin and see if they can access the advertiser
		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: ag.ExpID, ID: adv.ExpID}},

		// sign in as a different agency and see if we can access the advertiser
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 401, nil},

		// sign in as a talent agency and see if we can access it
		{"POST", "/signIn", adminTalentAgencyReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 401, nil},
	} {
		tr.Run(t, rst)
	}
}

func TestTalentAgencyChain(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.2,
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Female:    true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "justinbieber",
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		// try to directly signup as an agency
		{"POST", "/signUp", ag, 401, nil},

		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},
		{"POST", "/signIn", M{"email": ag.Email, "pass": defaultPass}, 200, nil},

		// change the agency's name and fee and check if it stuck
		{"GET", "/talentAgency/" + ag.ExpID, nil, 200, M{"fee": 0.2, "inviteCode": common.GetCodeFromID(ag.ExpID)}},
		{"PUT", "/talentAgency/" + ag.ExpID, &auth.User{TalentAgency: &auth.TalentAgency{ID: ag.ExpID, Name: "X", Fee: 0.3, Status: true}}, 200, nil},
		{"GET", "/talentAgency/" + ag.ExpID, nil, 200, M{"fee": 0.3, "inviteCode": common.GetCodeFromID(ag.ExpID)}},

		// create a new influencer as the new agency and signin
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		// update the influencer and check if the update worked
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "kimkardashian"}, 200, nil},
		// auditing as influencer should yield err
		{"PUT", "/setAudit/" + inf.ExpID, nil, 401, nil},
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		{"PUT", "/setAudit/" + inf.ExpID, M{"categories": []string{"business"}}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   ag.ExpID,
			"categories": []string{"business"},
			"twitter":    M{"id": "kimkardashian"},
		}},

		// sign in as admin and see if they can access the influencer
		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, nil},

		// sign in as a different agency and see if we can access the influencer
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 401, nil},

		// sign in as a talent agency and see if we can access it
		{"POST", "/signIn", adminTalentAgencyReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 401, nil},
	} {
		tr.Run(t, rst)
	}
}

func TestNewAdvertiser(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:  0.5,
		CCLoad:  creditCard,
		SubLoad: getSubscription(3, 100, true),
	}

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{}

	badAdv := getSignupUser()
	badAdv.Advertiser = &auth.Advertiser{
		DspFee: 2,
	}

	subUserEmail := adv.ExpID + "-login@test.org"
	subUser := M{"email": subUserEmail, "pass": "12345678"}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp?autologin=true", adv, 200, misc.StatusOK(adv.ExpID)},

		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: auth.SwayOpsAdAgencyID, DspFee: 0.5}},
		{"PUT", "/advertiser/" + adv.ExpID, &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.2, Plan: 3}}, 200, nil},

		// add a sub user and try to login with it
		{"POST", "/subUsers/" + adv.ExpID, subUser, 200, M{"id": adv.ExpID}},
		{"GET", "/subUsers/" + adv.ExpID, nil, 200, []string{subUserEmail}},
		{"POST", "/signIn", subUser, 200, nil},

		// try to add a sub user as a sub user
		{"POST", "/subUsers/" + adv.ExpID, subUser, 401, nil},

		// this used to fail, so testing for it in case a future change breaks it again
		{"GET", "/user", nil, 200, M{"id": adv.ExpID}},

		// log back in as the main adv
		{"POST", "/signIn", adv, 200, nil},

		// delete the subuser and try to log back in as it
		{"DELETE", "/subUsers/" + adv.ExpID + "/" + subUserEmail, nil, 200, nil},
		{"POST", "/signIn", subUser, 400, nil},

		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{DspFee: 0.2}},

		// add sub user as admin
		{"POST", "/subUsers/" + adv.ExpID, subUser, 200, M{"id": adv.ExpID}},

		{"GET", "/subUsers/" + adv.ExpID, nil, 200, []string{subUserEmail}},

		// create a new agency
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, nil},

		// test if a random agency can access it
		{"POST", "/signIn", M{"email": ag.Email, "pass": defaultPass}, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 401, nil},

		{"POST", "/signUp", badAdv, 400, nil},
	} {
		tr.Run(t, rst)
	}

	counter--

	if df, ef := getAdvertiserFees(srv.auth, adv.ExpID); df != 0.2 || ef != 0.2 {
		t.Fatal("getAdvertiserFees failed", df, ef)
	}

}

func TestNewInfluencer(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:        true,
			Female:      true,
			Geo:         &geo.GeoRecord{},
			TwitterId:   "justinbieber",
			InstagramId: "kimkardashian",
			YouTubeId:   "jennamarbles",
		},
	}

	badInf := getSignupUser()
	badInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Categories: []string{"BAD CAT"},
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		// update
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"PUT", "/setAudit/" + inf.ExpID, M{"categories": []string{"business"}}, 200, nil},
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "kimkardashian"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"categories": []string{"business"},
			"twitter":    M{"id": "kimkardashian"},
		}},

		// Add a social media platofrm
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "kimkardashian", "facebook": "justinbieber"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"categories": []string{"business"},
			"twitter":    M{"id": "kimkardashian"},
			"facebook":   M{"id": "justinbieber"},
		}},

		// change their password
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "kimkardashian", "facebook": "justinbieber", "oldPass": defaultPass, "pass": "newPassword", "pass2": "newPassword"}, 200, nil},
		// try to sign in.. should fail
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 400, nil},
		// try with proper password
		{"POST", "/signIn", M{"email": inf.Email, "pass": "newPassword"}, 200, nil},

		// try to load it as a different user
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 401, nil},

		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, nil},

		{"POST", "/signUp", badInf, 400, nil},
	} {
		tr.Run(t, rst)
	}

	var cats []*InfCategory
	r := rst.DoTesting(t, "GET", "/getCategories", nil, &cats)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cats) != len(common.CATEGORIES) {
		t.Fatal("No categories!")
		return
	}

	// this decreases the user id counter since the user id didn't increase in the
	// server because of the bad gender error in badInf.
	counter--
}

func TestInviteCode(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.2,
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Female:     true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "justinbieber",
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		// sign up as a new influencer and see if you get placed under above agency via invite code
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},

		// check influencer's agency
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},
	} {

		tr.Run(t, rst)
	}
}

func TestCampaigns(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{
		IsIO: true,
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee: 0.5,
	}
	cmp := common.Campaign{
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "American Beauty",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
	}
	cmpUpdate1 := `{"name":"Blade V","budget":150,"status":true,"tags":["mmmm"],"link":"blade.org","female": true,"instagram":true}`
	cmpUpdate2 := `{"advertiserId": "` + adv.ExpID + `", "name":"Blade VI?","budget":150,"status":true,"tags":["mmmm"],"link":"blade.org","female": true,"instagram":true}`
	badAdvId := cmp
	badAdvId.AdvertiserId = "1"

	for _, tr := range [...]*resty.TestRequest{
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		{"POST", "/signUp?autologin=true", ag, 200, misc.StatusOK(ag.ExpID)},

		{"POST", "/signUp?autologin=true", adv, 200, misc.StatusOK(adv.ExpID)},

		{"GET", "/advertiser/" + adv.ExpID, nil, 200, M{
			"id":       adv.ExpID,
			"agencyId": ag.ExpID,
		}},

		{"POST", "/campaign?dbg=1", &cmp, 200, nil},
		{"PUT", "/campaign/1", cmpUpdate1, 200, nil},
		{"GET", "/campaign/1", nil, 200, M{
			"name":         "Blade V",
			"agencyId":     ag.ExpID,
			"advertiserId": adv.ExpID,
		}},

		// access the campaign with the agency
		{"POST", "/signIn", M{"email": ag.Email, "pass": defaultPass}, 200, nil},
		{"PUT", "/campaign/1", cmpUpdate2, 200, nil},
		{"GET", "/campaign/1", nil, 200, M{"name": "Blade VI?"}},

		// sign in as a different ad agency and try to access the campaign
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/campaign/1", nil, 401, nil},

		// try to create a campaign with a bad advertiser id
		{"POST", "/campaign?dbg=1", &badAdvId, 400, nil},
	} {
		tr.Run(t, rst)
	}
}

func TestDeals(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.1,
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "breakingnews",
		},
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:  0.2,
		CCLoad:  creditCard,
		SubLoad: getSubscription(3, 100, true),
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       5000.5,
		Name:         "Get the supplies Rick",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
	}

	for _, tr := range [...]*resty.TestRequest{
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new talent agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		// sign up as a new influencer and see if you get placed under above agency via invite code
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},

		// check influencer's agency
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},

		// sign in as admin again
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new advertiser and sign in
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		// create a new campaign
		{"POST", "/campaign?dbg=1", &cmp, 200, nil},
		// sign in as influencer and get deals for the influencer
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		{"GET", "/getDeals/" + inf.ExpID + "/0/0", nil, 200, `{"campaignId": "2"}`},

		// assign yourself a deal
		{"GET", "/assignDeal/" + inf.ExpID + "/2/0/twitter?dbg=1", nil, 200, nil},

		// check deal assigned in influencer and campaign
		{"GET", "/influencer/" + inf.ExpID, nil, 200, `{"activeDeals":[{"campaignId": "2"}]}`},
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"GET", "/campaign/2?deals=true", nil, 200, resty.PartialMatch(fmt.Sprintf(`"influencerId":"%s"`, inf.ExpID))},

		// force approve the deal
		{"GET", "/forceApprove/" + inf.ExpID + "/2", nil, 200, misc.StatusOK(inf.ExpID)},

		// make sure it's put into completed
		{"GET", "/influencer/" + inf.ExpID, nil, 200, `{"completedDeals":[{"campaignId": "2"}]}`},
	} {
		tr.Run(t, rst)
	}

	verifyDeal(t, "2", inf.ExpID, ag.ExpID, rst, false)

	// Lets have a look at the forecast!
	var forecast struct {
		Breakdown   []*ForecastUser `json:"breakdown"`
		Reach       int64           `json:"reach"`
		Influencers int64           `json:"influencers"`
	}
	r := rst.DoTesting(t, "POST", "/getForecast?breakdown=250", &cmp, &forecast)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	if len(forecast.Breakdown) == 0 || forecast.Reach == 0 || forecast.Influencers == 0 {
		t.Fatal("Bad forecast values!")
		return
	}

	fInf := forecast.Breakdown[0]
	if fInf.Email == "" || fInf.Name == "" || fInf.ID == "" {
		t.Fatal("Bad forecast data!")
		return
	}

	if fInf.AvgEngs == 0 || fInf.Followers == 0 || fInf.ProfilePicture == "" || fInf.URL == "" {
		t.Fatal("Bad forecast twitter data!")
		return
	}

	// Sign up as a second influencer and do a deal! Need to see
	// cumulative stats
	newInf := getSignupUser()
	newInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "CNN",
			Categories: []string{"business"},
			BrandSafe:  "t",
		},
	}
	for _, tr := range [...]*resty.TestRequest{
		// sign up as a new influencer and see if you get placed under above agency via invite code
		{"POST", "/signUp", newInf, 200, misc.StatusOK(newInf.ExpID)},

		// check influencer's agency
		{"GET", "/influencer/" + newInf.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},

		{"GET", "/getDeals/" + newInf.ExpID + "/0/0", nil, 200, `{"campaignId": "2"}`},

		// assign yourself a deal
		{"GET", "/assignDeal/" + newInf.ExpID + "/2/0/twitter?dbg=1", nil, 200, nil},

		// check deal assigned in influencer and campaign
		{"GET", "/influencer/" + newInf.ExpID, nil, 200, `{"activeDeals":[{"campaignId": "2"}]}`},
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"GET", "/campaign/2?deals=true", nil, 200, resty.PartialMatch(fmt.Sprintf(`"influencerId":"%s"`, newInf.ExpID))},

		// force approve the deal
		{"GET", "/forceApprove/" + newInf.ExpID + "/2", nil, 200, misc.StatusOK(newInf.ExpID)},

		// make sure it's put into completed
		{"GET", "/influencer/" + newInf.ExpID, nil, 200, `{"completedDeals":[{"campaignId": "2"}]}`},
	} {
		tr.Run(t, rst)
	}

	verifyDeal(t, "2", newInf.ExpID, ag.ExpID, rst, true)

	// The first user was incomplete ("no categories")
	// They should be returned in get incomplete influencers
	var pendingInf []IncompleteInfluencer
	r = rst.DoTesting(t, "GET", "/getIncompleteInfluencers", nil, &pendingInf)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(pendingInf) == 0 {
		t.Fatal("Bad pending influencer length!")
		return
	}

	for _, i := range pendingInf {
		if i.TwitterURL == "" {
			t.Fatal("Unexpected twitter URL")
			return
		}

		if i.Id == newInf.ExpID {
			t.Fatal("Unexpected influencer in incomplete list!")
			return
		}
	}

	// Check reporting for just the second influencer
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+newInf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}
	var breakdownB map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+newInf.ExpID+"/10", nil, &breakdownB)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	checkReporting(t, breakdownB, 0, load.CompletedDeals[0], true, false)

	// Verify combined reporting because campaign reports will include both
	var newStore budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/2", nil, &newStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var breakdownA map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+inf.ExpID+"/10", nil, &breakdownA)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	totalA := breakdownA["total"]
	totalB := breakdownB["total"]

	totalShares := totalA.Shares + totalB.Shares
	totalLikes := totalA.Likes + totalB.Likes
	totalSpend := totalA.Spent + totalB.Spent

	// Difference between influencer stats and budget store is
	// talent agency + dsp + exchange

	// Lets remove the dsp and exchange fee and compare
	rawSpend := newStore.Spent - (2 * (0.2 * newStore.Spent))
	var agencyCut float64
	if agencyCut = (rawSpend - totalSpend) / rawSpend; agencyCut > 0.12 || agencyCut < 0.08 {
		t.Fatal("Combined spend does not match budget db!")
		return
	}

	var breakdownAgency1 map[string]*reporting.ReportStats
	r = rst.DoTesting(t, "GET", "/getAgencyInfluencerStats/"+ag.ExpID+"/"+inf.ExpID+"/10", nil, &breakdownAgency1)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var breakdownAgency2 map[string]*reporting.ReportStats
	r = rst.DoTesting(t, "GET", "/getAgencyInfluencerStats/"+ag.ExpID+"/"+newInf.ExpID+"/-1", nil, &breakdownAgency2)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(breakdownAgency2) > 1 {
		t.Fatal("Should only have total key!")
		return
	}

	// Combining totals for both influencers
	totalAgency1 := breakdownAgency1["total"]
	totalAgency2 := breakdownAgency2["total"]
	if statsSpend := totalAgency1.AgencySpent + totalAgency1.Spent + totalAgency2.AgencySpent + totalAgency2.Spent; int32(statsSpend) != int32(rawSpend) {
		t.Fatal("Unexpected spend values!")
		return
	}

	if totalAgency1.AgencySpent == totalAgency2.AgencySpent {
		t.Fatal("Issue with agency payout")
		return
	}

	if totalAgency1.AgencySpent > totalAgency1.Spent {
		t.Fatal("Agency spend higher than influencer spend!")
		return
	}

	var loads []*influencer.Influencer
	r = rst.DoTesting(t, "GET", "/getInfluencersByAgency/"+ag.ExpID, nil, &loads)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(loads) != 2 {
		t.Fatal("Incorrect number of infleuncers in agency!")
		return
	}
	for _, i := range loads {
		if i.Id == inf.ExpID {
			if i.AgencySpend != totalAgency1.AgencySpent {
				t.Fatal("Incorrect agency spend!")
				return
			}

			if i.InfluencerSpend != totalAgency1.Spent {
				t.Fatal("Incorrect inf spend!")
				return
			}
		} else if i.Id == newInf.ExpID {
			if i.AgencySpend != totalAgency2.AgencySpent {
				t.Fatal("Incorrect agency spend!")
				return
			}

			if i.InfluencerSpend != totalAgency2.Spent {
				t.Fatal("Incorrect inf spend!")
				return
			}
		}
	}

	var cmpBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignStats/2/10", nil, &cmpBreakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	totalCmp := cmpBreakdown["total"]
	if delta := totalCmp.Spent - newStore.Spent; delta > 0.25 || delta < -0.25 {
		t.Fatal("Combined spend does not match campaign report!")
		return
	}

	if totalCmp.Shares != totalShares {
		t.Fatal("Combined shares do not match campaign report!")
		return
	}

	if totalCmp.Likes != totalLikes {
		t.Fatal("Combined likes do not match campaign report!")
		return
	}

	if totalCmp.Influencers != 2 {
		t.Fatal("Influencer count incorrect!")
		return
	}

	var cmpInfBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignInfluencerStats/2/"+newInf.ExpID+"/10", nil, &cmpInfBreakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	totalCmpInf := cmpInfBreakdown["total"]
	if totalCmpInf.Likes != totalB.Likes {
		t.Fatal("Combined likes do not match campaign report!")
		return
	}

	if totalCmpInf.Shares != totalB.Shares {
		t.Fatal("Combined shares do not match campaign report!")
		return
	}

	var advDeals []*FeedCell
	r = rst.DoTesting(t, "GET", "/getAdvertiserContentFeed/"+adv.ExpID, nil, &advDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(advDeals) != 2 {
		t.Fatal("Expected 2 deals!")
		return
	}

	for _, advDeal := range advDeals {
		if advDeal.Views == 0 || advDeal.URL == "" || advDeal.Caption == "" || advDeal.SocialImage == "" {
			t.Fatal("Messed up deal!")
			return
		}
	}

	// Lets ban this influencer and create a campaign and see if they get any deals!
	r = rst.DoTesting(t, "GET", "/advertiserBan/"+adv.ExpID+"/"+inf.ExpID, nil, &advDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Lets make sure that the blacklist is there!
	var advertiser auth.Advertiser
	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(advertiser.Blacklist) != 1 {
		t.Fatal("Did not append to blacklist!")
		return
	}

	if _, ok := advertiser.Blacklist[inf.ExpID]; !ok {
		t.Fatal("Did not append to blacklist!")
		return
	}

	// Lets make sure that the blacklist is applied in all campaigns!
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/2", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if _, ok := cmpLoad.Blacklist[inf.ExpID]; !ok {
		t.Fatal("Did not append to blacklist!")
		return
	}

	// Lets create a campaign and make sure adv blacklist is applied on that
	cmpBlacklist := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       1000,
		Name:         "Watchu Talkin About Willis",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "http://www.cnn.com?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpBlacklist, &status)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var nextCmp common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+status.ID, nil, &nextCmp)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if _, ok := nextCmp.Blacklist[inf.ExpID]; !ok {
		t.Fatal("Did not append to blacklist!")
		return
	}
}

func verifyDeal(t *testing.T, cmpId, infId, agId string, rst *resty.Client, skipReporting bool) {
	var oldStore budget.Store
	r := rst.DoTesting(t, "GET", "/getBudgetInfo/"+cmpId, nil, &oldStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}
	checkStore(t, &oldStore, nil)

	// deplete budget according to the payout
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// check for money in completed deals (fees and inf)
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+infId, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}
	if len(load.CompletedDeals) != 1 {
		t.Fatal("Could not find completed deal!")
		return
	}

	doneDeal := load.CompletedDeals[0]

	checkDeal(t, doneDeal, &load, agId, cmpId)

	var newStore budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cmpId, nil, &newStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if newStore.Spent == 0 {
		t.Fatal("Spent not incremented correctly in budget")
		return
	}
	checkStore(t, &newStore, &oldStore)

	// check for money in campaign deals (fees and inf)
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cmpId+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	doneDeal = cmpLoad.Deals[load.CompletedDeals[0].Id]
	if doneDeal == nil {
		t.Fatal("Cannot find done deal in campaign!")
		return
	}

	checkDeal(t, doneDeal, &load, agId, cmpId)

	if !skipReporting {
		// check get campaign stats
		var breakdown map[string]*reporting.Totals
		r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cmpId+"/10", nil, &breakdown)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		checkReporting(t, breakdown, newStore.Spent, doneDeal, false, true)

		// check get influencer stats
		r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+infId+"/10", nil, &breakdown)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}
		checkReporting(t, breakdown, newStore.Spent, doneDeal, false, false)
	}
}

func checkStore(t *testing.T, store, compareStore *budget.Store) {
	if compareStore != nil {
		oldV := store.Spent + store.Spendable
		newV := compareStore.Spendable + compareStore.Spent
		if int32(newV) != int32(oldV) {
			t.Fatal("Spendable and spent not synchronized!")
		}

		if store.Spent == compareStore.Spent {
			t.Fatal("Spent did not change!")
		}
	}
}

func checkDeal(t *testing.T, doneDeal *common.Deal, load *influencer.Influencer, agId, campaignId string) {
	if doneDeal.CampaignId != campaignId {
		t.Fatal("Campaign ID not assigned!")
	}

	if doneDeal.Assigned == 0 || doneDeal.Completed == 0 {
		t.Fatal("Deal timestamps missing!")
	}

	if doneDeal.InfluencerId != load.Id {
		t.Fatal("Deal ID missing!")
	}

	var m *common.Stats
	if m = doneDeal.GetMonthStats(0); m != nil {
		if m.AgencyId != agId {
			t.Fatal("Payout to wrong talent agency!")
		}
		if m.Influencer == 0 {
			t.Fatal("No influencer payout!")

		}
		if m.Agency == 0 {
			t.Fatal("No agency payout!")
		}

		// Should be 10% as stated earlier on talent agency initialization
		if agencyFee := m.Agency / (m.Agency + m.Influencer); agencyFee > 0.11 || agencyFee < 0.09 {
			t.Fatal("Unexpected agency fee", agencyFee)
		}
	} else {
		t.Fatal("Could not find completed deal payout!")
	}

	if load.PendingPayout != m.Influencer {
		t.Fatal("Unexpected pending payout!")
	}

	if m = doneDeal.GetMonthStats(1); m.Influencer != 0 {
		t.Fatal("How the hell are you getting payouts from last month?")
	}
}

func checkReporting(t *testing.T, breakdown map[string]*reporting.Totals, spend float64, doneDeal *common.Deal, skipSpend, cmp bool) {
	report := breakdown["total"]
	dayTotal := breakdown[common.GetDate()]
	rt := int32(doneDeal.Tweet.Retweets)

	if rt != dayTotal.Shares || rt != report.Shares {
		t.Fatal("Shares do not match!")
	}

	likes := int32(doneDeal.Tweet.Favorites)
	if likes != dayTotal.Likes || likes != report.Likes {
		t.Fatal("Likes do not match!")
	}

	if !skipSpend {
		if report.Spent != spend {
			m := doneDeal.GetMonthStats(0)
			if cmp {
				// If we're comparing campaign stats.. it's spend includes markup!
				if int32(report.Spent) != int32(m.Influencer+m.Agency+m.DSP+m.Exchange) {
					t.Fatal("Campaign spend values do not match!")
				}
			} else {
				// Influencer stats should only have the influencer payout!
				if int32(report.Spent) != int32(m.Influencer) {
					t.Fatal("Influencer spend values do not match!")
				}
			}
		}
	}
}

func TestTaxes(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.1,
	}

	inf := getSignupUserWithEmail("shahzilsway@gmail.com") // throw away email
	inf.InfluencerLoad = &auth.InfluencerLoad{             // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "breakingnews",
			Address: &lob.AddressLoad{
				AddressOne: "8 Saint Elias",
				City:       "Trabuco Canyon",
				State:      "CA",
				Country:    "US",
			},
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new talent agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		// sign up as a new influencer and see if you get placed under above agency via invite code
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},

		// check influencer's agency
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},
	} {
		tr.Run(t, rst)
	}

	// Check reporting for just the second influencer
	var load influencer.Influencer
	r := rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code for initial check!")
	}

	if load.SignatureId != "" || load.HasSigned {
		t.Fatal("Unexpected signing!")
	}

	r = rst.DoTesting(t, "GET", "/emailTaxForm/"+inf.ExpID, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code for email!")
	}

	var uload influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &uload)
	if r.Status != 200 {
		t.Fatal("Bad status code second influencer check!")
	}

	if uload.SignatureId == "" {
		t.Fatal("No signature id assigned!")
	}

	if uload.RequestedTax == 0 {
		t.Fatal("No tax request timestamp!")
		return
	}

	val, err := hellosign.HasSigned(inf.ExpID, uload.SignatureId)
	if val || err != nil {
		t.Fatal("Error getting signed value!")
		return
	}

	// Cleanup
	time.Sleep(5 * time.Second)
	if r, err := hellosign.Cancel(uload.SignatureId); err != nil || r != 200 {
		t.Fatal("Hellosign cancel error")
		return
	}
}

func TestPerks(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.1,
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "breakingnews",
			Address: &lob.AddressLoad{
				AddressOne: "8 Saint Elias",
				City:       "Trabuco Canyon",
				State:      "CA",
				Country:    "US",
			},
		},
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150.5,
		Name:         "Warriors blew a 3-1 lead",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Perks:        &common.Perk{Name: "Nike Air Shoes", Type: 1, Count: 5},
	}

	for _, tr := range [...]*resty.TestRequest{
		// sign in as admin
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new talent agency and sign in
		{"POST", "/signUp", ag, 200, misc.StatusOK(ag.ExpID)},

		// sign up as a new influencer and see if you get placed under above agency via invite code
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},

		// check influencer's agency
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId": ag.ExpID,
		}},

		// sign in as admin again
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new advertiser and sign in
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		// create a new campaign
		{"POST", "/campaign", &cmp, 200, nil},

		// sign in as admin again
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
	} {
		tr.Run(t, rst)
	}

	// make sure number of deals allowed = number of perks
	var cmpLoad common.Campaign
	r := rst.DoTesting(t, "GET", "/campaign/4?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if len(cmpLoad.Deals) != cmp.Perks.Count && !*genData {
		t.Fatal("Unexpected number of deals!")
		return
	}

	if cmp.IsValid() {
		t.Fatal("Campaign should not be valid!")
		return
	}

	// Lets try increasing the number of perks!
	cmpUpdate := CampaignUpdate{
		Geos:       cmp.Geos,
		Categories: cmp.Categories,
		Status:     &cmp.Status,
		Budget:     &cmp.Budget,
		Male:       &cmp.Male,
		Female:     &cmp.Female,
		Name:       &cmp.Name,
		Perks:      &common.Perk{Name: "Nike Air Shoes", Type: 1, Count: 9},
	}

	r = rst.DoTesting(t, "PUT", "/campaign/4", &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check the number of deals and shit!
	r = rst.DoTesting(t, "GET", "/campaign/4?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	// It should be equal to the value it was before since new products
	// are in pending count!
	if len(cmpLoad.Deals) != cmp.Perks.Count {
		t.Fatal("Unexpected number of deals!")
		return
	}

	if cmpLoad.Perks.Count != cmp.Perks.Count {
		t.Fatal("Unexpected number of perks!")
		return
	}

	if cmpLoad.Perks.PendingCount != 4 {
		t.Fatal("Unexpected number of pending perks!")
		return
	}

	// make sure influencer getting no deals since the campaign is still pending
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals("4", deals)
	if len(deals) > 0 {
		t.Fatal("Unexpected number of deals!")
		return
	}

	// check admin endpoints for campaign approval
	var cmps []*common.Campaign
	r = rst.DoTesting(t, "GET", "/getPendingCampaigns", nil, &cmps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmps) != 1 {
		t.Fatal("Unexpected number of pending campaigns")
		return
	}

	if cmps[0].Id != "4" {
		t.Fatal("Unexpected campaign id!")
		return
	}

	if cmps[0].Approved > 0 {
		t.Fatal("Unexpected approval value!")
		return
	}

	// lets see if this campaign has any influencers that
	// could do a deal for them.. should be zero since it's not
	// approved!
	var cmpDeals []*DealOffer
	r = rst.DoTesting(t, "GET", "/getDealsForCampaign/4", nil, &cmpDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpDeals) > 0 {
		t.Fatal("Should be zero eligible deals!")
		return
	}

	var cmpPerks common.Campaign

	if *genData {
		goto SKIP_APPROVE_1
	}

	// approve campaign
	r = rst.DoTesting(t, "GET", "/approveCampaign/4", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// verify pending campaigns is empty!
	r = rst.DoTesting(t, "GET", "/getPendingCampaigns", nil, &cmps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmps) != 0 {
		t.Fatal("Pending campaigns shouldnt be here!")
		return
	}

	// make sure approved value is correct!
	r = rst.DoTesting(t, "GET", "/campaign/4", nil, &cmpPerks)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpPerks.Approved == 0 {
		t.Fatal("Admin approval didnt work!")
		return
	}

	// Lets make sure perk count is 9 now!
	if cmpPerks.Perks.PendingCount != 0 {
		t.Fatal("pending count incorrect!")
		return
	}

	if cmpPerks.Perks.Count != 9 {
		t.Fatal("pending count incorrect!")
		return
	}

SKIP_APPROVE_1:

	// Lets make sure there are deals for
	// this campaign now!
	r = rst.DoTesting(t, "GET", "/getDealsForCampaign/4", nil, &cmpDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpDeals) == 0 {
		t.Fatal("Expected campaign deals!")
		return
	}

	found := false
	for _, offer := range cmpDeals {
		if offer.Influencer.Id == inf.ExpID {
			found = true
		}
	}
	if !found {
		t.Fatal("No campaign deal with expected influencer!")
		return
	}

	// get deals for influencer
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals("4", deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals!")
		return
	}

	for _, d := range deals {
		if d.CampaignImage == "" {
			t.Fatal("Missing image url!")
			return
		}
	}

	tgDeal := deals[0]
	if tgDeal.CampaignId != "4" {
		t.Fatal("Unexpected campaign id!")
	}

	if tgDeal.Perk == nil {
		t.Fatal("Should have a perk attached!")
	}

	if tgDeal.Perk.Count != 1 {
		t.Fatal("Incorrect reporting of perk count")
	}

	if tgDeal.Perk.InfId != "" || tgDeal.Perk.Status {
		t.Fatal("Incorrect perk values set!")
	}

	// lets make sure the getDeal endpoint works and has
	// all necessary data!
	var dealGet common.Deal
	r = rst.DoTesting(t, "GET", "/getDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id, nil, &dealGet)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if dealGet.Spendable == 0 || len(dealGet.Platforms) == 0 || dealGet.Perk.Count != 1 || dealGet.Link == "" {
		t.Fatal("Get deal query did not work!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id+"/twitter", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// make sure get deal still works!
	var postDeal common.Deal
	r = rst.DoTesting(t, "GET", "/getDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id, nil, &postDeal)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if postDeal.Spendable == 0 || len(postDeal.Platforms) == 0 || postDeal.Link == "" {
		t.Fatal("Get deal query did not work post assignment!")
	}

	// check campaign perk status and count (make sure both were updated)
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(load.ActiveDeals) != 1 {
		t.Fatal("Unexpected number of active deals!")
	}

	tgDeal = load.ActiveDeals[0]
	if tgDeal.Perk == nil {
		t.Fatal("No perk assigned!")
	}

	if tgDeal.Perk.Status {
		t.Fatal("Unexpected perk status!")
	}

	if tgDeal.Perk.InfId != inf.ExpID {
		t.Fatal("Incorrect inf id set for perk!")
	}

	if tgDeal.Perk.Address == nil {
		t.Fatal("No address set for perk!")
	}

	if tgDeal.CampaignId != "4" {
		t.Fatal("Unexpected campaign id for deal!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/4?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if cmpLoad.Perks == nil {
		t.Fatal("Campaign has no perks man!")
	}

	if cmpLoad.Perks.Count != 8 {
		t.Fatal("Campaign perk count did not decrement!")
	}

	cmpDeal, ok := cmpLoad.Deals[tgDeal.Id]
	if !ok || cmpDeal.Perk == nil {
		t.Fatal("Unexpected campaign deal value!")
	}

	if cmpDeal.Perk.InfId != inf.ExpID {
		t.Fatal("Influencer ID not assigned to campaign deal")
	}

	if cmpDeal.Perk.Address == nil {
		t.Fatal("Unexpected deal address")
	}

	if cmpDeal.Perk.Status {
		t.Fatal("Campaign deal should not be approved yet!")
	}

	// get pending perk sendouts for admin
	var pendingPerks []PerkWithCmpInfo

	r = rst.DoTesting(t, "GET", "/getPendingPerks", nil, &pendingPerks)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(pendingPerks) != 1 {
		t.Fatal("Unexpected number of perks.. should have 1!", len(pendingPerks))
	}

	if len(pendingPerks) != 1 {
		t.Fatal("Unexpected number of perks")
	}

	if pendingPerks[0].CampaignID != "4" {
		t.Fatal("Unknown perk request campaign ID!")
	}

	if pendingPerks[0].Status {
		t.Fatal("Incorrect perk status value!")
	}

	if pendingPerks[0].Address == nil {
		t.Fatal("No address for perk!")
	}

	var emptyPerks []PerkWithCmpInfo

	if *genData {
		goto SKIP_APPROVE_2
	}

	// approve sendout
	r = rst.DoTesting(t, "GET", "/approvePerk/"+inf.ExpID+"/4", nil, &pendingPerks)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// make sure get pending perk doesnt have that perk request now
	r = rst.DoTesting(t, "GET", "/getPendingPerks", nil, &emptyPerks)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(emptyPerks) != 0 {
		t.Fatal("Pending perk still leftover!")
	}

SKIP_APPROVE_2:

	// make sure status is now true on campaign and influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	tgDeal = load.ActiveDeals[0]
	if tgDeal.Perk == nil {
		t.Fatal("No perk assigned!")
	}

	if !tgDeal.Perk.Status {
		t.Fatal("Deal perk status should be true!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/4?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpDeal, ok = cmpLoad.Deals[tgDeal.Id]
	if !ok || cmpDeal.Perk == nil {
		t.Fatal("Unexpected campaign deal value!")
	}

	if !cmpDeal.Perk.Status {
		t.Fatal("Campaign deal should be approved now!")
	}

	if *genData {
		return
	}

	// force approve
	r = rst.DoTesting(t, "GET", "/forceApprove/"+inf.ExpID+"/4", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// verify deal
	verifyDeal(t, "4", inf.ExpID, ag.ExpID, rst, false)

	// Lets try increasing the number of perks!
	cmpUpdate = CampaignUpdate{
		Geos:       cmpLoad.Geos,
		Categories: cmpLoad.Categories,
		Status:     &cmpLoad.Status,
		Budget:     &cmpLoad.Budget,
		Male:       &cmpLoad.Male,
		Female:     &cmpLoad.Female,
		Name:       &cmpLoad.Name,
		Perks:      &common.Perk{Name: "Nike Air Shoes", Type: 1, Count: 15},
	}

	r = rst.DoTesting(t, "PUT", "/campaign/4", &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check the number of deals and shit!
	r = rst.DoTesting(t, "GET", "/campaign/4?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if len(cmpLoad.Deals) != cmpUpdate.Perks.Count-cmpLoad.Perks.PendingCount {
		t.Fatal("Unexpected number of deals!")
		return
	}

	if cmpLoad.Perks.Count+1 != cmpUpdate.Perks.Count-cmpLoad.Perks.PendingCount {
		t.Fatal("Unexpected number of perks!")
		return
	}

	// Create a campaign with coupon perks
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150.5,
		Name:         "A coupon code campaign",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Task:         "Post pictures with your new Nike shoes!",
		Tags:         []string{"#mmmm"},
		Perks:        &common.Perk{Name: "Nike Air Shoes Coupons", Count: 5}, // No type so should error!
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
	if r.Status == 200 {
		// Should reject!
		t.Fatalf("Bad status code: %s", r.Value)
		return
	}

	if !strings.Contains(string(r.Value), "Invalid perk type") {
		t.Fatal("Bad err msg")
		return
	}

	cmp.Perks.Type = 2 // Set coupon type

	// Should now reject because we didn't pass any coupon codes!
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
	if r.Status == 200 {
		// Should reject!
		t.Fatalf("Bad status code: %s", r.Value)
		return
	}

	if !strings.Contains(string(r.Value), "Please provide coupon codes") {
		t.Fatal("Bad err msg")
		return
	}

	cmp.Perks.Codes = []string{"123COUPON", "321COUPON", "LASTCOUPON"}
	cmp.Perks.Instructions = "Go to nike.com and use the coupon at check out!"

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Make sure it's approved and has other expected values
	var couponCmp common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &couponCmp)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if couponCmp.Approved == 0 {
		t.Fatal("Coupon campaign should be approved!")
		return
	}

	if couponCmp.Perks.Count != len(cmp.Perks.Codes) {
		t.Fatal("Bad coupon count!")
		return
	}

	if len(couponCmp.Deals) != couponCmp.Perks.Count {
		t.Fatal("Bad deal count!")
		return
	}

	// Lets try increasing the number of perks!
	updatedCodes := append(cmp.Perks.Codes, "LASTLASTCOUPON")
	cmpUpdate = CampaignUpdate{
		Geos:       cmp.Geos,
		Categories: cmp.Categories,
		Status:     &cmp.Status,
		Budget:     &cmp.Budget,
		Male:       &cmp.Male,
		Female:     &cmp.Female,
		Name:       &cmp.Name,
		Perks:      &common.Perk{Type: 2, Codes: updatedCodes},
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check the number of deals and shit!
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &couponCmp)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if len(couponCmp.Deals) != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of deals!", len(cmpUpdate.Perks.Codes), len(couponCmp.Deals))
		return
	}

	if couponCmp.Perks.Count != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of perks!")
		return
	}

	// Lets try DECREASING the number of perks!
	cmpUpdate = CampaignUpdate{
		Geos:       cmp.Geos,
		Categories: cmp.Categories,
		Status:     &cmp.Status,
		Budget:     &cmp.Budget,
		Male:       &cmp.Male,
		Female:     &cmp.Female,
		Name:       &cmp.Name,
		Perks:      &common.Perk{Type: 2, Codes: updatedCodes[:len(updatedCodes)-1]},
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check the number of deals and shit!
	var freshCampaign common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &freshCampaign)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if len(freshCampaign.Deals) != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of deals!", len(cmpUpdate.Perks.Codes), len(freshCampaign.Deals))
		return
	}

	if freshCampaign.Perks.Count != len(updatedCodes)-1 {
		t.Fatal("Unexpected number of perks!")
		return
	}

	if freshCampaign.Perks.Count != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of perks!")
		return
	}

	// Make sure it doesnt show up in admin approval
	var couponCmps []*common.Campaign
	r = rst.DoTesting(t, "GET", "/getPendingCampaigns", nil, &couponCmps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	for _, cmp := range couponCmps {
		if cmp.Id == st.ID {
			t.Fatal("WTF COUPON CAMPAIGN SHOULDNT BE IN ADMIN APPROVAL LIST")
			return
		}
	}

	// Get the deal and make sure it has the right category
	var couponDeals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &couponDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals(st.ID, couponDeals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals!")
		return
	}

	deal := deals[0]
	if deal.Perk == nil {
		t.Fatal("No perk!")
		return
	}

	if deal.Perk.Instructions != cmp.Perks.Instructions {
		t.Fatal("Bad instructions")
		return
	}

	if len(deal.Perk.Codes) > 0 {
		t.Fatal("No coupon codes should be visible!")
		return
	}

	if deal.Perk.Count != 1 {
		t.Fatal("Bad perk count")
		return
	}

	if deal.Perk.Category != "Coupon" {
		t.Fatal("Bad category")
		return
	}

	if deal.Perk.InfId != "" || deal.Perk.Status {
		t.Fatal("Incorrect perk values set!")
		return
	}

	// Assign yourself the deal and make sure you have a coupon code
	var doneDeal common.Deal
	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+st.ID+"/"+deal.Id+"/twitter?dbg=1", nil, &doneDeal)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if doneDeal.Perk.Instructions != cmp.Perks.Instructions {
		t.Fatal("No instructions")
		return
	}

	if doneDeal.Perk.Code != "LASTCOUPON" {
		t.Fatal("Bad coupon code")
		return
	}

	var updCampaign *common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &updCampaign)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	var fnd *common.Deal
	for _, deal := range updCampaign.Deals {
		if deal.Id == doneDeal.Id {
			fnd = deal
			break
		}
	}

	if fnd == nil {
		t.Fatal("Deal not found")
		return
	}

	if fnd.Assigned == 0 {
		t.Fatal("WTF it should be assigned to the influencer")
		return
	}

	if !fnd.Perk.Status {
		t.Fatal("Perk status should be true")
	}

	if freshCampaign.Perks.Count != (updCampaign.Perks.Count + 1) {
		t.Fatal("Count did not decrease by one")
		return
	}

	for _, code := range updCampaign.Perks.Codes {
		if code == "LASTLASTCOUPON" {
			t.Fatal("Coupon code did not delete")
			return
		}
	}

	// Lets add a new coupon given that there's already one deal assigned
	updatedCodes = append(updCampaign.Perks.Codes, "THEFINALCOUNTDOWN")
	cmpUpdate = CampaignUpdate{
		Geos:       updCampaign.Geos,
		Categories: updCampaign.Categories,
		Status:     &updCampaign.Status,
		Budget:     &updCampaign.Budget,
		Male:       &updCampaign.Male,
		Female:     &updCampaign.Female,
		Name:       &updCampaign.Name,
		Perks:      &common.Perk{Type: 2, Codes: updatedCodes},
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check the number of deals and shit!
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &couponCmp)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %+v", r)
		return
	}

	if len(updCampaign.Deals) != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of deals!", len(updCampaign.Deals), len(cmpUpdate.Perks.Codes))
		return
	}

	if updCampaign.Perks.Count+1 != len(cmpUpdate.Perks.Codes) {
		t.Fatal("Unexpected number of perks!")
		return
	}
}

func getDeals(cid string, deals []*common.Deal) []*common.Deal {
	out := []*common.Deal{}
	for _, d := range deals {
		if d.CampaignId == cid {
			out = append(out, d)
		}
	}
	return out
}

func TestInfluencerEmail(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUserWithEmail("shahzilabid@gmail.com")
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code! %s", r.Value)
	}

	// Lets check default email value
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if !load.DealPing {
		t.Fatal("Incorrect deal ping value!")
	}

	// Lets set this influencer to NOT receive emails now!
	offPing := false
	updLoad := &InfluencerUpdate{DealPing: &offPing, TwitterId: "cnn"}
	r = rst.DoTesting(t, "PUT", "/influencer/"+inf.ExpID, updLoad, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets try forcing an email run
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Influencer's email ping setting is off.. they should have
	// no email sent!
	var latestLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &latestLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if latestLoad.DealPing || latestLoad.LastEmail != 0 {
		t.Fatal("Unexpected email values for influencer")
	}

	// Lets set this influencer to receive emails now!
	onPing := true
	updLoad = &InfluencerUpdate{DealPing: &onPing, TwitterId: "cnn"}
	r = rst.DoTesting(t, "PUT", "/influencer/"+inf.ExpID, updLoad, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var lastLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &lastLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if !lastLoad.DealPing {
		t.Fatal("Deal ping did not toggle!")
	}

	// Create some campaigns so the influencer gets an email!
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}
	for i := 1; i < 8; i++ {
		cmp := common.Campaign{
			Status:       true,
			AdvertiserId: adv.ExpID,
			Budget:       float64(i + 150),
			Name:         "Auto Campaign " + strconv.Itoa(i),
			Twitter:      true,
			Male:         true,
			Female:       true,
			Link:         "blade.org",
			Task:         "POST THAT DOPE SHIT " + strconv.Itoa(i) + " TIMES!",
			Tags:         []string{"#mmmm"},
		}
		r := rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
		if r.Status != 200 {
			t.Fatalf("Bad status code: %s", r.Value)
		}
	}

	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Should have an email now!
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if load.LastEmail == 0 {
		t.Fatal("No email was sent wth!", inf.ExpID)
	}

	// Lets force email again and make sure a new email doesnt get sent
	// since we haven't reached threshold yet
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var newLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &newLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if load.LastEmail != newLoad.LastEmail {
		t.Fatal("A new email was sent.. HOW?!")
	}
}

func TestImages(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create the campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp := common.Campaign{
		AdvertiserId: adv.ExpID,
		Budget:       150.5,
		Name:         "The Day Walker",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %s", r.Value)
	}

	// Make sure the default image url was set
	var load common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/1", nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if !strings.Contains(load.ImageURL, "/images/campaign/default") {
		t.Fatal("Incorrect default image set!")
	}

	r = rst.DoTesting(t, "GET", load.ImageURL, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets try hitting an image which doesnt exist
	r = rst.DoTesting(t, "GET", "/images/campaign/default_dne.jpg", nil, nil)
	if r.Status == 200 {
		t.Fatal("Bad status code!")
	}

	// Try uploading a bad image
	r = rst.DoTesting(t, "PUT", "/campaign/1", smallImage, nil)
	if r.Status != 400 {
		t.Fatalf("Bad status code: %s", r.Value)
	}

	// Upload correct image
	r = rst.DoTesting(t, "PUT", "/campaign/1", goodImage, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
	}

	// Make sure image url now correct
	r = rst.DoTesting(t, "GET", "/campaign/1", nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if load.ImageURL == "" || strings.Contains(load.ImageURL, "default") {
		t.Fatal("Incorrect image url!")
	}

	r = rst.DoTesting(t, "GET", load.ImageURL, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Remove saved image (and indirectly check if it exists)!
	err := os.Remove("./" + load.ImageURL)
	if err != nil {
		t.Fatal("File does not exist!", ".."+load.ImageURL)
	}
}

func TestInfluencerGeo(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			IP:        "72.229.28.185", // NYC
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Influencer should have a geo set using the IP
	var load geo.GeoRecord
	r = rst.DoTesting(t, "GET", "/getLatestGeo/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if load.Source != "ip" {
		t.Fatal("Bad source for geo!")
	}

	if load.State != "ny" || load.Country != "us" {
		t.Fatal("Incorrect geo set using IP!")
	}

	// Lets update the address for the influencer now!
	addr := lob.AddressLoad{
		AddressOne: "8 Saint Elias",
		City:       "Trabuco Canyon",
		State:      "CAlifornia",
		Country:    "US",
	}

	updLoad := &InfluencerUpdate{Address: addr, TwitterId: "cnn"}
	r = rst.DoTesting(t, "PUT", "/influencer/"+inf.ExpID, updLoad, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Influencer should have a geo set using the address
	r = rst.DoTesting(t, "GET", "/getLatestGeo/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if load.Source != "address" {
		t.Fatal("Bad source for geo!")
	}

	if load.State != "CA" || load.Country != "US" {
		t.Fatal("Incorrect geo set using IP!")
	}

	// Create a campaign with bad geo
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Bad geo!
	fakeGeo := []*geo.GeoRecord{
		&geo.GeoRecord{State: "ASDFASDF", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
		&geo.GeoRecord{Country: "GB"},
	}
	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Geo Campaign",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
	if r.Status == 200 {
		t.Fatal("Bad status code!")
	}

	// All correct geo.. lets try creating a proper campaign now!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
		&geo.GeoRecord{Country: "GB"},
	}
	cmp.Geos = fakeGeo
	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Check to make sure geos in campaign all good!
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(cmpLoad.Geos) != 3 {
		t.Fatal("Unexpected number of geos!")
	}

	// Lets see if our California influencer gets a deal with this campaign!
	// They should!
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals.. should have atleast one!")
	}

	// Update campaign with geo that doesnt match our California influencer!
	cmpUpdateGood := `{"geos": [{"state": "TX", "country": "US"}, {"country": "GB"}], "name":"Blade V","budget":150,"status":true,"tags":["mmmm"],"male":true,"female":true,"twitter":true}`
	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, cmpUpdateGood, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(cmpLoad.Geos) != 2 {
		t.Fatal("Unexpected number of geos!")
	}

	// Influencer should no longer get a deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) != 0 {
		t.Fatal("Unexpected number of deals!")
	}

	// Lets try a UK user who should get a deal!
	// Create a UK influencer

	// NOTE: Add this test once we allow non US/CA campaigns

	// ukInf := getSignupUser()
	// ukInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
	// 	InfluencerLoad: influencer.InfluencerLoad{
	// 		IP:        "131.228.17.26", // London
	// 		TwitterId: "cnn",
	// 	},
	// }
	// r = rst.DoTesting(t, "POST", "/signUp", &ukInf, nil)
	// if r.Status != 200 {
	// 	t.Fatal("Bad status code!")
	// }

	// // Influencer should get a deal
	// r = rst.DoTesting(t, "GET", "/getDeals/"+ukInf.ExpID+"/0/0", nil, &deals)
	// if r.Status != 200 {
	// 	t.Fatal("Bad status code!")
	// }

	// deals = getDeals(st.ID, deals)
	// if len(deals) == 0 {
	// 	t.Fatal("Unexpected number of deals!")
	// }
}

func TestChecks(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "justinbieber",
			Address: &lob.AddressLoad{
				AddressOne: "8 Saint Elias",
				City:       "Trabuco Canyon",
				State:      "CA",
				Country:    "US",
			},
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Do multiple deals
	for i := 0; i < 8; i++ {
		doDeal(rst, t, inf.ExpID, "2", true)
	}

	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(load.CompletedDeals) != 8 {
		t.Fatal("Unexpected numnber of completed deals!")
	}

	// Lets do a get for pending checks.. should be none since no infs have
	// requested yet
	var checkInfs []*GreedyInfluencer
	r = rst.DoTesting(t, "GET", "/getPendingChecks", nil, &checkInfs)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(checkInfs) > 0 {
		t.Fatal("Unexpected number of check requested influencers!")
	}

	// request a check!
	r = rst.DoTesting(t, "GET", "/requestCheck/"+inf.ExpID+"?skipTax=1", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/getPendingChecks", nil, &checkInfs)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(checkInfs) != 1 {
		t.Fatal("Unexpected number of check requested influencers!")
	}

	if checkInfs[0].Id != inf.ExpID {
		t.Fatal("Wrong influencer ID!")
	}

	if len(checkInfs[0].CompletedDeals) != 8 {
		t.Fatal("Incorrect number of completed deals!")
	}

	if checkInfs[0].CompletedDeals[0] == "" {
		t.Fatal("Incorrect post URL!")
	}

	if checkInfs[0].Followers == 0 {
		t.Fatal("Wrong influencer ID!")
	}

	if checkInfs[0].RequestedCheck == 0 {
		t.Fatal("Wrong request check timestamp!")
	}

	if checkInfs[0].PendingPayout < 50 {
		t.Fatal("Wrong pending payout!")
	}

	if checkInfs[0].Address == nil {
		t.Fatal("Missing address!")
	}

	if *genData {
		return
	}

	// Approve the check!
	r = rst.DoTesting(t, "GET", "/approveCheck/"+inf.ExpID, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var approvedInf auth.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &approvedInf)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if approvedInf.PendingPayout != 0 {
		t.Fatal("Unexpected pending payout")
	}

	if len(approvedInf.Payouts) != 1 {
		t.Fatal("Unexpected number of checks")
	}

	if int32(approvedInf.Payouts[0].Payout) != int32(checkInfs[0].PendingPayout) {
		t.Fatal("Unexpected payout history!")
	}

	if approvedInf.RequestedCheck != 0 {
		t.Fatal("Requested check value still exists!")
	}

	if !misc.WithinLast(approvedInf.LastCheck, 1) {
		t.Fatal("Last check value is incorrect!")
	}
}

type Status struct {
	ID string `json:"id"`
}

const DEFAULT_BUDGET = 1000

func doDeal(rst *resty.Client, t *testing.T, infId, agId string, approve bool) (cid string) {
	// Create a campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:   0.2,
		AgencyID: agId,
	}

	if agId != auth.SwayOpsTalentAgencyID {
		adv.Advertiser.CCLoad = creditCard
		adv.Advertiser.SubLoad = getSubscription(3, 100, true)
	}

	var st Status
	r := rst.DoTesting(t, "POST", "/signUp", adv, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: st.ID,
		Budget:       DEFAULT_BUDGET,
		Name:         "shiyeeeet",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "http://www.cnn.com?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &status)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}
	cid = status.ID

	// get deals for influencer
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+infId+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	deals = getDeals(cid, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals!")
		return
	}

	var doneDeal common.Deal
	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+infId+"/"+cid+"/"+deals[0].Id+"/twitter?dbg=1", nil, &doneDeal)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if doneDeal.ShortenedLink == "" {
		t.Fatal("Shortened link not created!")
		return
	}

	if !approve {
		return
	}

	// force approve
	r = rst.DoTesting(t, "GET", "/forceApprove/"+infId+"/"+cid+"", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// check that the deal is approved
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+infId, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(load.CompletedDeals) == 0 {
		t.Fatal("Unexpected numnber of completed deals!")
		return
	}

	// deplete budget according to the payout
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	return
}

func getTestClick(url string) string {
	idx := strings.Index(url, "/c/")
	return url[idx:]
}

func TestClicks(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Make sure click endpoint accessible without signing in
	r := rst.DoTesting(t, "GET", "/c/JxA", nil, nil)
	if r.Status == 401 {
		t.Fatal("Unexpected unauthorized error!")
		return
	}

	// Sign in as admin
	r = rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "cnn",
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
		return
	}

	// Do a deal but DON'T approve yet!
	doDeal(rst, t, inf.ExpID, "2", false)

	// Influencer has assigned deals.. lets try clicking!
	var load auth.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(load.ActiveDeals) != 1 && len(load.CompletedDeals) != 0 {
		t.Fatal("Unexpected number of deals")
		return
	}

	cid := load.ActiveDeals[0].CampaignId

	// Make sure the shortened url is saved correctly
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cid, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(cmpLoad.Link, "cnn.com") {
		t.Fatal("Shortening of the URL did not work!")
		return
	}

	if !strings.Contains(load.ActiveDeals[0].ShortenedLink, "/c/") {
		t.Fatal("Unexpected shortened link")
		return
	}

	// Try faking a click for an active deal.. should add a pending click and redirect!
	r = rst.DoTesting(t, "GET", getTestClick(load.ActiveDeals[0].ShortenedLink), nil, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(r.URL, "cnn.com") {
		t.Fatal("Incorrect redirect")
		return
	}

	// Make sure there are no clicks or uniques in reporting
	var breakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &breakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if breakdown["total"].Clicks > 0 {
		t.Fatal("Unexpected number of clicks!")
		return
	}

	if breakdown["total"].Uniques > 0 {
		t.Fatal("Unexpected number of uniques!")
		return
	}

	// There should be a pending click for the deal however!
	var pendingClick common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &pendingClick)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	clicks := 0
	for _, deal := range pendingClick.Deals {
		for _, stats := range deal.Reporting {
			clicks += len(stats.PendingClicks)
		}
	}

	if clicks != 1 {
		t.Fatal("Expected one pending click")
		return
	}

	// Approve the deal
	r = rst.DoTesting(t, "GET", "/forceApprove/"+inf.ExpID+"/"+cid+"", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// check that the deal is approved
	var newLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &newLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(newLoad.CompletedDeals) == 0 {
		t.Fatal("Unexpected numnber of completed deals!")
		return
	}

	// deplete budget according to the payout
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(getTestClick(newLoad.CompletedDeals[0].ShortenedLink), "/c/") {
		t.Fatal("Bad shortened link")
		return
	}

	// Try a real click
	r = rst.DoTesting(t, "GET", getTestClick(newLoad.CompletedDeals[0].ShortenedLink), nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if !strings.Contains(r.URL, "cnn.com") {
		t.Fatal("Incorrect redirect")
		return
	}

	// Make sure pending click is still there
	var approvedClick common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &approvedClick)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var pending, approved int
	for _, deal := range approvedClick.Deals {
		for _, stats := range deal.Reporting {
			pending += len(stats.PendingClicks)
			approved += len(stats.ApprovedClicks)
		}
	}

	if pending != 1 {
		t.Fatal("Expected 1 pending click")
		return
	}

	if approved != 1 {
		t.Fatal("Expected 1 approved click")
		return
	}

	// Make sure everything increments
	var newBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &newBreakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if newBreakdown["total"].Clicks != 1 {
		t.Fatal("Unexpected number of clicks!")
		return
	}

	if newBreakdown["total"].Uniques != 1 {
		t.Fatal("Unexpected number of uniques!")
		return
	}

	// // Try clicking again.. should fail because of unique check!
	// r = rst.DoTesting(t, "GET", getTestClick(newLoad.CompletedDeals[0].ShortenedLink), nil, nil)
	// if r.Status != 200 {
	// 	t.Fatal("Bad status code!")
	// 	return
	// }

	// if !strings.Contains(r.URL, "cnn.com") {
	// 	t.Fatal("Incorrect redirect")
	// 	return
	// }

	// // Make sure nothing increments since this uuid just had a click
	// var lastBreakdown map[string]*reporting.Totals
	// r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &lastBreakdown)
	// if r.Status != 200 {
	// 	t.Fatal("Bad status code!")
	// 	return
	// }

	// if lastBreakdown["total"].Clicks != 1 {
	// 	t.Fatal("Unexpected number of clicks!")
	// 	return
	// }

	// Lets try an actual click and make sure it shows up in pending and
	// stats remain as 1
	r = rst.DoTesting(t, "GET", getTestClick(newLoad.CompletedDeals[0].ShortenedLink)+"?dbg=1", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(r.URL, "cnn.com") {
		t.Fatal("Incorrect redirect")
		return
	}

	var lastBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &lastBreakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if lastBreakdown["total"].Clicks != 1 {
		t.Fatal("Unexpected number of clicks!")
		return
	}

	if lastBreakdown["total"].Uniques != 1 {
		t.Fatal("Unexpected number of Uniques!")
		return
	}

	// Make sure pending click is there
	var pendingCheck common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &pendingCheck)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	pending, approved = 0, 0
	for _, deal := range pendingCheck.Deals {
		for _, stats := range deal.Reporting {
			pending += len(stats.PendingClicks)
			approved += len(stats.ApprovedClicks)
		}
	}

	if pending != 2 {
		t.Fatal("Unexpected number of pending clicks!")
		return
	}

	if approved != 1 {
		t.Fatal("Unexpected number of approved clicks!")
		return
	}

	// Lets run depletion.. should transfer over clicks!
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var lastCheck common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &lastCheck)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	pending, approved = 0, 0
	for _, deal := range lastCheck.Deals {
		for _, stats := range deal.Reporting {
			pending += len(stats.PendingClicks)
			approved += len(stats.ApprovedClicks)
		}
	}

	if pending != 0 {
		t.Fatal("Unexpected number of pending clicks!")
		return
	}

	if approved != 3 {
		t.Fatal("Unexpected number of approved clicks!")
		return
	}

	r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &lastBreakdown)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if lastBreakdown["total"].Clicks != 3 {
		t.Fatal("Unexpected number of clicks!")
		return
	}

	if lastBreakdown["total"].Uniques != 1 {
		t.Fatal("Unexpected number of uniques!")
		return
	}
}

type Count struct {
	Count int `json:"count"`
}

func TestScraps(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create some scraps
	scraps := []influencer.Scrap{}
	scraps = append(scraps, influencer.Scrap{
		Name:         "UCWJ2lWNubArHWmf3FIHbfcQ",
		YouTube:      true,
		EmailAddress: "blah23@a.b",
	})

	scraps = append(scraps, influencer.Scrap{
		Name:         "UCWJ2lWNubArHWmf3FIHbfcQ",
		YouTube:      true,
		EmailAddress: "blah24@a.b",
	})

	scraps = append(scraps, influencer.Scrap{
		Name:         "nba",
		Instagram:    true,
		EmailAddress: "blah25@a.b",
	})

	r = rst.DoTesting(t, "POST", "/setScrap", &scraps, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create a campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:   0.2,
		AgencyID: "2",
		CCLoad:   creditCard,
		SubLoad:  getSubscription(3, 100, true),
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/signUp", adv, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: st.ID,
		Budget:       1000,
		Name:         "Sick of coming up with campaign names",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "http://www.cnn.com?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Force scrap email
	var count Count
	r = rst.DoTesting(t, "GET", "/forceScrapEmail", nil, &count)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Only Tig Bitties should get an email!
	if count.Count != 1 {
		t.Fatal("Didn't email right amount of scraps!")
		return
	}

	// Lets get all scraps to verify values
	var getScraps []*influencer.Scrap
	r = rst.DoTesting(t, "GET", "/getScraps", nil, &getScraps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(getScraps) != 3 {
		t.Fatal("Wrong number of scraps!")
		return
	}

	for _, sc := range getScraps {
		if sc.Name == "nba" {
			if len(sc.SentEmails) != 1 {
				t.Fatal("Low number of sent emails")
				return
			}
		} else {
			if len(sc.SentEmails) > 0 {
				t.Fatal("High number of sent emails")
				return
			}
		}
	}

	// Lets run a scrap email again.. values should be the same
	// since we haven't hit the 48 hour threshold for second email!
	var newCount Count
	r = rst.DoTesting(t, "GET", "/forceScrapEmail", nil, &newCount)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Only Tig Bitties should have gotten an email!
	if newCount.Count != 0 {
		t.Fatal("Didn't email right amount of scraps!")
		return
	}

	// Verify values again!
	var lastScraps []*influencer.Scrap
	r = rst.DoTesting(t, "GET", "/getScraps", nil, &lastScraps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(lastScraps) != 3 {
		t.Fatal("Wrong number of scraps!")
		return
	}

	for _, sc := range lastScraps {
		if sc.Name == "nba" {
			if len(sc.SentEmails) != 1 {
				t.Fatal("Low number of sent emails")
				return
			}
		} else {
			if len(sc.SentEmails) > 0 {
				t.Fatal("High number of sent emails")
				return
			}
		}
	}

	r = rst.DoTesting(t, "GET", "/forceAttributer", nil, &count)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Lets create a new influencer with that email and handle..
	// they should get the same keywords!
	// Create an influencer with same credential as scrap
	inf := getSignupUserWithEmail("blah25@a.b")
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male: true,
			Geo:  &geo.GeoRecord{},
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code! %s", r.Value)
	}

	updLoad := &InfluencerUpdate{InstagramId: "nba"}
	r = rst.DoTesting(t, "PUT", "/influencer/"+inf.ExpID, updLoad, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(load.Keywords) == 0 {
		t.Fatal("Bad keywords")
	}

	if !strings.Contains(load.Keywords[0], "old") {
		t.Fatal("Bad keyword!")
	}
}

func TestBalances(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create a campaign with non IO agency
	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{}
	r = rst.DoTesting(t, "POST", "/signUp", ag, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	adv := getSignupUser()
	adv.ParentID = ag.ExpID
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp := common.Campaign{
		Status:       false,
		AdvertiserId: adv.ExpID,
		Budget:       230,
		Name:         "Insert cool campaign name",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Since we just spawned a campaign with off status.. it should have nothing
	// in spendable and no budget store
	var store budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+st.ID, nil, &store)
	if r.Status != 500 {
		// Expecting a failed hit because it should have no budget
		t.Fatal("Bad status code!")
		return
	}

	// Make sure it DOESNT have deals
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmp)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmp.Deals) != 0 {
		t.Fatal("Should NOT have deals now")
		return
	}

	// Toggle the campaign ON
	updStatus := true
	cmpUpdate := CampaignUpdate{
		Geos:       cmp.Geos,
		Categories: cmp.Categories,
		Status:     &updStatus,
		Budget:     &cmp.Budget,
		Male:       &cmp.Male,
		Female:     &cmp.Female,
		Name:       &cmp.Name,
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID+"?dbg=1", &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check if it has a budget key now.. it should
	var store1 budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+st.ID, nil, &store1)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if store1.Spendable == 0 {
		t.Fatal("Should have spendable")
		return
	}

	if len(store1.Charges) != 1 {
		t.Fatal("Missing charges")
		return
	}

	// Toggle the campaign OFF again
	updStatus = false
	cmpUpdate = CampaignUpdate{
		Geos:       cmp.Geos,
		Categories: cmp.Categories,
		Status:     &updStatus,
		Budget:     &cmp.Budget,
		Male:       &cmp.Male,
		Female:     &cmp.Female,
		Name:       &cmp.Name,
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID+"?dbg=1", &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check if it has a budget key now.. it should NOT
	var storeToggle budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+st.ID, nil, &storeToggle)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if storeToggle.Spendable != 0 {
		t.Fatal("Should NOT have spendable")
		return
	}

	if len(store1.Charges) != 1 {
		t.Fatal("Missing charges")
		return
	}

	// Make sure it has deals
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmp)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmp.Deals) == 0 {
		t.Fatal("Should have deals now")
		return
	}

	// Lets start a campaign as ON and make sure it has data
	cmpOn := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       15000,
		Name:         "Insert cool campaign name that is on",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var stOn Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpOn, &stOn)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Make sure it has deals
	r = rst.DoTesting(t, "GET", "/campaign/"+stOn.ID+"?deals=true", nil, &cmpOn)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpOn.Deals) == 0 {
		t.Fatal("Should have deals now")
		return
	}

	// Lets make sure it has budget info
	var storeOn budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+stOn.ID, nil, &storeOn)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if storeOn.Spendable == 0 {
		t.Fatal("Should have spendable")
		return
	}

	// Lets switch the campaign to off.. it's spendable should disappear
	updStatus = false
	cmpUpdate = CampaignUpdate{
		Geos:       cmpOn.Geos,
		Categories: cmpOn.Categories,
		Status:     &updStatus,
		Budget:     &cmpOn.Budget,
		Male:       &cmpOn.Male,
		Female:     &cmpOn.Female,
		Name:       &cmpOn.Name,
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+stOn.ID+"?dbg=1", &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets check if it has a budget key now.. it should have NO spendable
	var storeEmpty budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+stOn.ID, nil, &storeEmpty)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if storeEmpty.Spendable != 0 {
		t.Fatal("Spendable should have been emptied out")
		return
	}

	// if storeEmpty.Budget == 0 {
	// 	t.Fatal("Should have carried over budget value")
	// 	return
	// }

	if len(storeEmpty.Charges) == 0 {
		t.Fatal("Should have charges")
		return
	}

	// Lets make sure that spendable was moved to balances
	var advBillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv.ExpID, nil, &advBillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if advBillingInfo.InactiveBalance == 0 {
		t.Fatal("Bad balance")
		return
	}

	if advBillingInfo.InactiveBalance != storeOn.Spendable {
		t.Fatal("Spendable did not transfer to balance")
		return
	}

	// Start a campaign with a small budget.. we should use
	// the balance for it
	cmpSmall := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       300,
		Name:         "Insert cool campaign name that is small",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var stSmall Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpSmall, &stSmall)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var storeSmall budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+stSmall.ID, nil, &storeSmall)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var usedBillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv.ExpID, nil, &usedBillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if usedBillingInfo.InactiveBalance == 0 {
		t.Fatal("Bad balance")
		return
	}

	if usedBillingInfo.InactiveBalance != (advBillingInfo.InactiveBalance - storeSmall.Spendable) {
		t.Fatal("Did not deduct from balance correctly")
		return
	}

	if usedBillingInfo.ActiveBalance == 0 {
		t.Fatal("No active balance")
		return
	}

	if usedBillingInfo.ActiveBalance != (storeSmall.Spendable + storeSmall.Spent) {
		t.Fatal("Active balance should be combined sum")
		return
	}

	// Lets a crazy high amount via campaign creation.. a part of it should
	// be used from balance and the rest via charging the CC
	cmpBig := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       30000,
		Name:         "Insert cool campaign name that is massive",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var stBig Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpBig, &stBig)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var storeBig budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+stBig.ID, nil, &storeBig)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(storeBig.Charges) == 0 {
		t.Fatal("Should have charges")
		return
	}

	if storeBig.Spendable == 0 {
		t.Fatal("Should have spendable")
		return
	}

	if (storeBig.Charges[0].FromBalance + storeBig.Charges[0].Amount) != storeBig.Spendable {
		t.Fatal("Spendable calculation incorrect")
		return
	}

	// Balance should be 0
	var bigBillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv.ExpID, nil, &bigBillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if bigBillingInfo.InactiveBalance != 0 {
		t.Fatal("Bad balance")
		return
	}

	if len(bigBillingInfo.History) == 0 {
		t.Fatal("No billing history")
		return
	}

	if bigBillingInfo.ActiveBalance != (storeBig.Spendable + storeSmall.Spendable) {
		t.Fatal("Active balance doesn't add up")
		return
	}

	var foundBalance bool
	for _, ch := range bigBillingInfo.History {
		if ch.FromBalance != "0.00" {
			foundBalance = true
		}
	}

	if !foundBalance {
		t.Fatal("Could not find from balance on swipe")
		return
	}
}

func TestInfluencerClearout(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}
	// Create a campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Insert cool campaign name",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see if our influencer gets a deal with this campaign!
	// They should!
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals.. should have atleast one!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+st.ID+"/"+deals[0].Id+"/twitter", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// lets make sure there is an assigned deal
	var found bool
	for _, deal := range cmpLoad.Deals {
		if deal.IsActive() && deal.InfluencerId == inf.ExpID {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("No active deals!")
	}

	// Toggle the campaign off
	cmpUpdateGood := `{"geos": [{"state": "TX", "country": "US"}, {"country": "GB"}], "name":"Blade V","budget":150,"status":false,"tags":["mmmm"],"male":true,"female":true,"twitter":true}`
	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID+"?dbg=1", cmpUpdateGood, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets make sure it has no active deals
	var load common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(load.Deals) == 0 {
		t.Fatal("No deals at all!")
	}

	found = false
	for _, deal := range load.Deals {
		if deal.IsActive() && len(deal.Platforms) == 0 {
			found = true
		}
	}

	if found {
		t.Fatal("Shouldn't have active deals!")
	}

	var infLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &infLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	found = false
	for _, deal := range infLoad.ActiveDeals {
		if deal.CampaignId == st.ID {
			found = true
		}
	}

	if found {
		t.Fatal("Shouldn't have active deals!")
	}
}

func TestStripe(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{}
	r = rst.DoTesting(t, "POST", "/signUp", ag, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets try updating credit card information and make sure it works
	adv1 := getSignupUser()
	adv1.ParentID = ag.ExpID
	adv1.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var advertiser1 auth.Advertiser
	r = rst.DoTesting(t, "GET", "/advertiser/"+adv1.ExpID, nil, &advertiser1)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var adv1BillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv1.ExpID, nil, &adv1BillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}
	if adv1BillingInfo.CreditCard == nil {
		t.Fatal("No credit card assigned")
	}

	if adv1BillingInfo.CreditCard.FirstName != creditCard.FirstName {
		t.Fatal("Wrong name")
	}

	advUpd1 := &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, Customer: advertiser1.Customer, CCLoad: newCreditCard}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv1.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var upd1BillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv1.ExpID, nil, &upd1BillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if upd1BillingInfo.CreditCard == nil {
		t.Fatal("No credit card assigned")
	}

	if upd1BillingInfo.CreditCard.FirstName != newCreditCard.FirstName {
		t.Fatal("Name not updated")
	}

	// Make sure the customer id is still the same
	if upd1BillingInfo.ID != adv1BillingInfo.ID {
		t.Fatal("Customer ID wrongfully updated")
	}

	// Lets try deleting the credit card!
	advUpd1 = &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, Customer: advertiser1.Customer, CCLoad: &swipe.CC{Delete: true}}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv1.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var delBillingInfo BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv1.ExpID, nil, &delBillingInfo)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if delBillingInfo.CreditCard != nil {
		t.Fatal("Credit card still there")
	}

	// Make sure the customer id is still the same
	if delBillingInfo.ID != adv1BillingInfo.ID {
		t.Fatal("Customer ID wrongfully updated")
	}

	// Create a campaign WITHOUT Agency having IO status but a CC attached!
	adv := getSignupUser()
	adv.ParentID = ag.ExpID
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "The Stripe Walker",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// This should have charged the stripe card for the above campaign!
	var bHist BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv.ExpID, nil, &bHist)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	hist := bHist.History
	if len(hist) != 1 {
		t.Fatal("Missing billing history")
	}

	if hist[0].Name != cmp.Name {
		t.Fatal("Non-matching campaign name")
	}

	if hist[0].ID != st.ID {
		t.Fatal("Non-matching campaign ID")
	}

	if hist[0].Amount > uint64(cmp.Budget*100) || hist[0].Amount == 0 {
		t.Fatal("Unexpected amounts")
	}

	// Lets creating a second campaign and make sure both charges show up
	cmp2 := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       1000,
		Name:         "The Night Crawler",
		Twitter:      true,
		Male:         true,
		Link:         "nba.com",
		Task:         "POST THAT DOPE SHIT MAYN",
		Tags:         []string{"#mmmmDonuts"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp2, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var newHist BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+adv.ExpID, nil, &newHist)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	hist = newHist.History
	if len(hist) != 2 {
		t.Fatal("Missing billing history")
	}

	var hFound *swipe.History
	for _, h := range hist {
		if h.Name == cmp2.Name {
			hFound = h
			break
		}
	}

	log.Println("stripe user:", adv.ExpID)

	if hFound == nil {
		t.Fatal("Missing charge for campaign")
	}

	if hFound.ID != st.ID {
		t.Fatal("Non-matching campaign ID")
	}

	if hFound.Amount > uint64(cmp2.Budget*100) || hFound.Amount == 0 {
		t.Fatal("Unexpected amounts")
	}

	// Lets make sure budget store has NO charges
	var store budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+st.ID, nil, &store)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(store.Charges) != 1 {
		t.Fatal("Charge not recorded")
	}

	// Lets create an agency with NO IO STATUS and advertiser with no CC
	// and try creating a campaign (should error)
	agNoIO := getSignupUser()
	agNoIO.AdAgency = &auth.AdAgency{}
	r = rst.DoTesting(t, "POST", "/signUp", agNoIO, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	advNoCC := getSignupUser()
	advNoCC.ParentID = agNoIO.ExpID
	advNoCC.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", advNoCC, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpNoCC := common.Campaign{
		Status:       true,
		AdvertiserId: advNoCC.ExpID,
		Budget:       150,
		Name:         "No CC",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpNoCC, nil)
	if r.Status == 200 {
		t.Fatal("Unexpected status code!")
	}

	if !strings.Contains(string(r.Value), "Credit card not found") {
		t.Fatal("Unexpected error")
	}

	// Lets create an IO agency and adv with no CC and try creating a
	// campaign (should work)
	agIO := getSignupUser()
	agIO.AdAgency = &auth.AdAgency{
		IsIO: true,
	}
	r = rst.DoTesting(t, "POST", "/signUp", agIO, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	advIO := getSignupUser()
	advIO.ParentID = agIO.ExpID
	advIO.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", advIO, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpIO := common.Campaign{
		Status:       true,
		AdvertiserId: advIO.ExpID,
		Budget:       150,
		Name:         "No CC",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpIO, &st)
	if r.Status != 200 {
		t.Fatal("Unexpected status code!")
	}

	// Make sure theres nothing in billing history!
	var ioHist BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+advIO.ExpID, nil, &ioHist)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	hist = ioHist.History
	if len(hist) != 0 {
		t.Fatal("Where'd billing history come from?! Should have none")
	}

	// Lets use an IO agency WITH a CC adv and try creating a campaign (should work but no charge to stripe)
	advCC := getSignupUser()
	advCC.ParentID = agIO.ExpID
	advCC.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
	}
	r = rst.DoTesting(t, "POST", "/signUp", advCC, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpCC := common.Campaign{
		Status:       true,
		AdvertiserId: advCC.ExpID,
		Budget:       150,
		Name:         "No CC",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpCC, &st)
	if r.Status != 200 {
		t.Fatal("Unexpected status code!")
	}

	// Make sure theres nothing in billing history!
	var ccHist BillingInfo
	r = rst.DoTesting(t, "GET", "/billingInfo/"+advCC.ExpID, nil, &ccHist)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	hist = ccHist.History
	if len(hist) != 0 {
		t.Fatal("Where'd billing history come from?! Should have none")
	}

	// Lets verify the info that's saved for the user
	var suser auth.User
	r = rst.DoTesting(t, "GET", "/user/"+advCC.ExpID, nil, &suser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if suser.Advertiser == nil || suser.Advertiser.Customer == "" {
		t.Fatal("Missing stripe customer")
	}

	// Lets create a non-io agency and campaign
	// Then switch the agency to non IO, and create a campaign
	agIOSwitch := getSignupUser()
	agIOSwitch.AdAgency = &auth.AdAgency{
		IsIO: false,
	}
	r = rst.DoTesting(t, "POST", "/signUp", agIOSwitch, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	advIOSwitch := getSignupUser()
	advIOSwitch.ParentID = agIOSwitch.ExpID
	advIOSwitch.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
	}
	r = rst.DoTesting(t, "POST", "/signUp", advIOSwitch, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpIOSwitch := common.Campaign{
		Status:       true,
		AdvertiserId: advIOSwitch.ExpID,
		Budget:       150,
		Name:         "IO Switch",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var newSt Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpIOSwitch, &newSt)
	if r.Status != 200 {
		t.Fatal("Unexpected status code!")
	}

	// Lets verify that we have 1  charge
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+newSt.ID, nil, &store)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(store.Charges) != 1 {
		t.Fatal("Charge not recorded")
	}

	if store.Charges[0].Amount == 0 {
		t.Fatal("Amount value incorrect")
	}

	if store.Charges[0].Timestamp == 0 {
		t.Fatal("Timestamp value incorrect")
	}

	// Lets increase the budget and make sure NO charges show up
	// as a charge should only show up on billing day
	budgetVal := float64(5000)
	upd := &CampaignUpdate{Budget: &budgetVal}
	r = rst.DoTesting(t, "PUT", "/campaign/"+newSt.ID, upd, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+newSt.ID, nil, &store)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(store.Charges) != 1 {
		t.Fatal("Bad len on charges")
	}

	// Lets switch the agency to IO now and create a campaign
	agUpd := &auth.User{AdAgency: &auth.AdAgency{
		ID:     agIOSwitch.ExpID,
		IsIO:   true,
		Name:   "Post update",
		Status: true,
	}}

	r = rst.DoTesting(t, "PUT", "/adAgency/"+agIOSwitch.ExpID, agUpd, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
		return
	}

	// Lets make sure its been switched
	var newAg *auth.AdAgency
	r = rst.DoTesting(t, "GET", "/adAgency/"+agIOSwitch.ExpID, nil, &newAg)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !newAg.IsIO {
		t.Fatal("WTF WE JUST SWITCHED IT ON")
	}

	// Time to create a campaign with no CC
	advCCSwitch := getSignupUser()
	advCCSwitch.ParentID = agIOSwitch.ExpID
	advCCSwitch.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", advCCSwitch, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmpCCSwitch := common.Campaign{
		Status:       true,
		AdvertiserId: advCCSwitch.ExpID,
		Budget:       150,
		Name:         "CC Switch Campaign",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var switchSt Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmpCCSwitch, &switchSt)
	if r.Status != 200 {
		t.Fatal("Unexpected status code!")
	}

	// OK whew.. lets check charge now!
	var lastStore budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+switchSt.ID, nil, &lastStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(lastStore.Charges) != 0 {
		t.Fatal("Bad len on charges")
	}

	return
}

type KeywordCount struct {
	Keywords []string `json:"keywords"`
}

func TestAttributer(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var getScraps []*influencer.Scrap
	r = rst.DoTesting(t, "GET", "/getScraps", nil, &getScraps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create some scraps
	scraps := []influencer.Scrap{}
	scraps = append(scraps, influencer.Scrap{
		Name:         "UCWJ2lWNubArHWmf3FIHbfcQ",
		YouTube:      true,
		EmailAddress: "nba@a.b",
	})

	scraps = append(scraps, influencer.Scrap{
		Name:         "justinbieber",
		Twitter:      true,
		EmailAddress: "jb@a.b",
	})

	scraps = append(scraps, influencer.Scrap{
		Name:         "UCWJ2lWNubArHWmf3FIHbfcQ",
		YouTube:      true,
		EmailAddress: "jb@a.b",
	})

	scraps = append(scraps, influencer.Scrap{
		Name:         "angelicaalcalaherrera",
		Instagram:    true,
		EmailAddress: "insta@a.b",
	})

	r = rst.DoTesting(t, "POST", "/setScrap", &scraps, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// force attributer
	var count Count
	r = rst.DoTesting(t, "GET", "/forceAttributer", nil, &count)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if count.Count < 4 {
		t.Fatal("Not enough scraps updated!", count.Count)
		return
	}

	var updatedScraps []*influencer.Scrap
	r = rst.DoTesting(t, "GET", "/getScraps", nil, &updatedScraps)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if (len(updatedScraps) - len(getScraps)) != 4 {
		t.Fatal("No new scraps!")
		return
	}

	for _, sc := range getScraps {
		if sc.EmailAddress == "nba@a.b" || sc.EmailAddress == "insta@a.b" || sc.EmailAddress == "jb@a.b" {
			if sc.Updated == 0 {
				t.Fatal("Scrap should be attributed!")
				return
			}

			if sc.Followers == 0 {
				t.Fatal("Followers not set!")
				return
			}

			if len(sc.Keywords) == 0 {
				t.Fatal("No keywords set")
				return
			}

			// Sandbox always returns "computer" for keywords
			if sc.Keywords[0] != "god" {
				t.Fatal("Bad keywords set")
				return
			}

			if len(sc.Categories) != 1 {
				t.Fatal("Bad categories set")
				return
			}

			if sc.Categories[0] != "spirituality" {
				t.Fatal("Bad category set")
				return
			}

			if sc.Fails != 0 {
				t.Fatal("Incorrect fails")
				return
			}

		}
	}
}

func TestForceApprove(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create an influencer
	ag := getSignupUser()
	ag.TalentAgency = &auth.TalentAgency{
		Fee: 0.1,
	}

	r = rst.DoTesting(t, "POST", "/signUp", &ag, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "justinbieber",
		},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/signUp", &inf, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}
	infId := st.ID

	// Do a deal but DON'T approve yet!
	doDeal(rst, t, infId, "2", false)

	// Lets grab the influencer
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+infId, nil, &load)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(load.ActiveDeals) != 1 {
		t.Fatal("No active deals!")
		return
	}

	if load.Twitter == nil || len(load.Twitter.LatestTweets) == 0 {
		t.Fatal("Huh? Twitter feed empty!")
		return
	}

	cmpId := load.ActiveDeals[0].CampaignId

	// Lets try force approving via a bad URL.. should yield an error!
	fApp := &ForceApproval{
		URL:          "blahblah.com/test",
		Platform:     "twitter",
		InfluencerID: infId,
		CampaignID:   cmpId,
	}

	r = rst.DoTesting(t, "POST", "/forceApprovePost", &fApp, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	// Lets try a valid URL now!
	postURL := load.Twitter.LatestTweets[0].PostURL
	if postURL == "" {
		t.Fatal("Missing post URL!")
		return
	}

	fApp.URL = postURL
	r = rst.DoTesting(t, "POST", "/forceApprovePost", &fApp, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	verifyDeal(t, cmpId, infId, ag.ExpID, rst, false)

	return
}

func TestSubscriptions(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	/////////////////////////////////////
	//////// ENTERPRISE TESTING /////////
	/////////////////////////////////////

	// Create an advertiser with the ENTERPRISE subscription
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(subscriptions.ENTERPRISE, 125, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var advertiser auth.Advertiser
	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// The advertiser should have a subscription ID assigned and a plan ID
	if advertiser.Plan != subscriptions.ENTERPRISE {
		t.Fatal("Bad plan code!")
		return
	}

	if advertiser.Subscription == "" {
		t.Fatal("No subscription ID assigned!")
		return
	}

	// Lets make sure the price we sent is the one in stripe!
	price, monthly, err := swipe.GetSubscription(advertiser.Subscription)
	if err != nil {
		t.Fatal("Err on sub query")
		return
	}

	if price != 125 {
		t.Fatal("Bad sub price")
		return
	}

	if !monthly {
		t.Fatal("Bad sub interval")
		return
	}

	// Sign in as admin
	r = rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see how many sub users we can make for enterprise!
	for i := 1; i <= 10; i++ {
		subUserEmail := adv.ExpID + "-login@test.org" + strconv.Itoa(i)
		subUser := M{"email": subUserEmail, "pass": "12345678"}
		r = rst.DoTesting(t, "POST", "/subUsers/"+adv.ExpID, subUser, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!", string(r.Value), i)
			return
		}
	}

	// Lets create a campaign under ENTERPRISE with full capabilities!
	// Should work!
	fakeGeo := []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
		&geo.GeoRecord{Country: "GB"},
		&geo.GeoRecord{Country: "AW"},
	}
	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does all targeting",
		Twitter:      true,
		YouTube:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there are no errors with creating!
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Plan != subscriptions.ENTERPRISE {
		t.Fatal("Campaign plan incorrect!")
		return
	}

	t.Logf("enterprise subscription user: %s", adv.ExpID)

	/////////////////////////////////////
	//////// PREMIUM TESTING ////////////
	/////////////////////////////////////

	// Lets try a Premium plan and make sure our campaign filters work!
	adv = getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(subscriptions.PREMIUM, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// The advertiser should have a subscription ID assigned and a plan ID
	if advertiser.Plan != subscriptions.PREMIUM {
		t.Fatal("Bad plan code!")
		return
	}

	if advertiser.Subscription == "" {
		t.Fatal("No subscription ID assigned!")
		return
	}

	// Sign in as admin
	r = rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see how many sub users we can make for premium! Should be 5
	LIMIT := 5
	for i := 1; i <= 10; i++ {
		subUserEmail := adv.ExpID + "-login@test.org" + strconv.Itoa(i)
		subUser := M{"email": subUserEmail, "pass": "12345678"}
		r = rst.DoTesting(t, "POST", "/subUsers/"+adv.ExpID, subUser, nil)
		if i > LIMIT {
			if r.Status == 200 {
				t.Fatal("Bad status code!", string(r.Value), i)
				return
			}
		} else {
			if r.Status != 200 {
				t.Fatal("Bad status code!", string(r.Value), i)
				return
			}
		}
	}

	// Lets create a campaign that should be REJECTED based on what Premium plan
	// allows
	// NOTE: This campaign is targeting Aruba so should be rejected!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
		&geo.GeoRecord{Country: "GB"},
		&geo.GeoRecord{Country: "AW"},
	}
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does all targeting",
		Twitter:      true,
		YouTube:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there ARE errors with creating!
	if r.Status == 200 {
		t.Fatal("Expected rejection!")
	}

	// Lets create a campaign that should be ACCEPTED based on what Premium plan
	// allows
	// NOTE: This campaign is targeting Aruba so should be rejected!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
		&geo.GeoRecord{Country: "GB"},
	}
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does most targeting",
		Twitter:      true,
		YouTube:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there ARE errors with creating!
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Plan != subscriptions.PREMIUM {
		t.Fatal("Campaign plan incorrect!")
		return
	}

	t.Logf("premium subscription user: %s", adv.ExpID)

	//////////////////////////////////////
	//////// HYPER LOCAL TESTING /////////
	//////////////////////////////////////

	// Lets try a HYPER LOCAL plan and make sure our campaign filters work!
	adv = getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(subscriptions.HYPERLOCAL, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// The advertiser should have a subscription ID assigned and a plan ID
	if advertiser.Plan != subscriptions.HYPERLOCAL {
		t.Fatal("Bad plan code!")
		return
	}

	if advertiser.Subscription == "" {
		t.Fatal("No subscription ID assigned!")
		return
	}

	// Sign in as admin
	r = rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see how many sub users we can make for hyperlocal! Should be 0
	subUserEmail := adv.ExpID + "-login@test.org.hyper"
	subUser := M{"email": subUserEmail, "pass": "12345678"}
	r = rst.DoTesting(t, "POST", "/subUsers/"+adv.ExpID, subUser, nil)
	if r.Status == 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	// Lets create a campaign that should be REJECTED based on what Premium plan
	// allows
	// NOTE: This campaign is targeting Canada so should be rejected!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
		&geo.GeoRecord{State: "ON", Country: "CA"},
	}
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does all targeting",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there ARE errors with creating!
	if r.Status == 200 {
		t.Fatal("Expected rejection!")
	}

	// Lets create a campaign that should be REJECTED based on what Premium plan
	// allows
	// NOTE: This campaign is targeting Twitter and Youtube so should be rejected!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
	}
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does all targeting",
		Twitter:      true,
		YouTube:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there ARE errors with creating!
	if r.Status == 200 {
		t.Fatal("Expected rejection!")
	}

	// Lets create a campaign that should be ACCEPTED based on what Premium plan
	// allows
	// NOTE: This campaign is targeting Aruba so should be rejected!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "CA", Country: "US"},
	}
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Campaign that does most targeting",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	// Make sure there ARE errors with creating!
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Plan != subscriptions.HYPERLOCAL {
		t.Fatal("Campaign plan incorrect!")
		return
	}

	t.Logf("hyberlocal subscription user: %s", adv.ExpID)

	////////////////////////[/////////////
	//////// PLAN CHANGE TESTING /////////
	//////////////////////////////////////

	// Lets update a plan!

	// Lets start off with a PREMIUM plan
	adv = getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(subscriptions.PREMIUM, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if advertiser.Plan != subscriptions.PREMIUM {
		t.Fatal("Bad plan set!")
		return
	}

	oldSub := advertiser.Subscription
	if oldSub == "" {
		t.Fatal("Bad sub set")
		return
	}

	// Lets upgrade to enterprise!
	advUpd1 := &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, CCLoad: creditCard, SubLoad: getSubscription(subscriptions.ENTERPRISE, 100, true)}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		log.Println(string(r.Value))
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if advertiser.Subscription == "" {
		t.Fatal("Bad sub set")
		return
	}

	if advertiser.Plan != subscriptions.ENTERPRISE {
		t.Fatal("Bad plan set")
		return
	}

	if advertiser.Subscription == oldSub {
		t.Fatal("Sub not updated")
		return
	}

	oldSub = advertiser.Subscription

	// Lets downgrade to hyperlocal!
	advUpd1 = &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, CCLoad: creditCard, SubLoad: getSubscription(subscriptions.HYPERLOCAL, 100, true)}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &advertiser)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if advertiser.Subscription == "" {
		t.Fatal("Bad sub set")
		return
	}

	if advertiser.Plan != subscriptions.HYPERLOCAL {
		t.Fatal("Bad plan set")
		return
	}

	if advertiser.Subscription == oldSub {
		t.Fatal("Sub not updated")
		return
	}

	// Lets cancel the plan now!
	advUpd1 = &auth.User{Advertiser: &auth.Advertiser{Status: true, DspFee: 0.1, CCLoad: creditCard, Subscription: advertiser.Subscription, SubLoad: getSubscription(0, 0, true)}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// The subscription should be cancelled on stripe!
	_, _, err = swipe.GetSubscription(advertiser.Subscription)
	if err == nil {
		t.Fatal("Subscription should be cancelled!")
		return
	}

	var cancelledAdv auth.Advertiser
	r = rst.DoTesting(t, "GET", "/advertiser/"+adv.ExpID, nil, &cancelledAdv)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cancelledAdv.Subscription != "" {
		t.Fatal("Bad sub set")
		return
	}

	if cancelledAdv.Plan != 0 {
		t.Fatal("Bad plan set")
		return
	}

	// Lets create a hyperlocal advertiser.. NOT get a deal under them
	// for a big inf then lets upgrade the plan to enterprise and make sure
	// that deal appears! Then lets cancel the plan and make sure
	// deal disappears!

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			InstagramId: "kimkardashian",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}
	// Create a campaign
	adv = getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(subscriptions.HYPERLOCAL, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "DIS NOT DA ONE HOMIE",
		Instagram:    true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see if our influencer gets a deal with this campaign!
	// They should NOT!
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) > 0 {
		t.Fatal("Unexpected number of deals.. should have zero!")
	}

	// Lets switch the plan to enterprise.. should get deals now!
	advUpd1 = &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, CCLoad: creditCard, SubLoad: getSubscription(subscriptions.ENTERPRISE, 100, true)}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets make sure the campaign's plan value was updated!
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Plan != subscriptions.ENTERPRISE {
		t.Fatal("Campaign failed to update")
		return
	}

	// Lets see if our influencer gets a deal with this campaign!
	// They should!
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) != 1 {
		t.Fatal("Unexpected number of deals.. should have 1!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+st.ID+"/"+deals[0].Id+"/instagram", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Make sure deal is there!
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// lets make sure there is an assigned deal
	var found bool
	for _, deal := range cmpLoad.Deals {
		if deal.IsActive() && deal.InfluencerId == inf.ExpID {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("No active deals!")
		return
	}

	// Lets cancel the plan now!
	advUpd1 = &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.1, CCLoad: creditCard, Subscription: advertiser.Subscription, SubLoad: getSubscription(0, 0, true)}}
	r = rst.DoTesting(t, "PUT", "/advertiser/"+adv.ExpID, &advUpd1, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see if a deal appears for a cancelled subscription.. it shouldn't!
	// Create an influencer
	inf = getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			InstagramId: "selenagomez",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets see if our influencer gets a deal with this campaign!
	// They should NOT!
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals(st.ID, deals)
	if len(deals) > 0 {
		t.Fatal("Unexpected number of deals.. should have zero!")
	}
}

func TestTimeline(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}
	// Create a campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}

	t.Logf("timeline userID: %s", adv.ExpID)

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "Insert cool campaign name",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets make sure we got the correct timeline message
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpLoad.Timeline) != 1 {
		t.Fatal("Missing timeline attribute")
		return
	}

	if cmpLoad.Timeline[0].Message != common.CAMPAIGN_APPROVAL {
		t.Fatal("Bad message!")
		return
	}

	// approve campaign
	r = rst.DoTesting(t, "GET", "/approveCampaign/"+st.ID, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpLoad.Timeline) != 2 {
		t.Fatal("Missing timeline attribute")
		return
	}

	if cmpLoad.Timeline[1].Message != common.CAMPAIGN_START {
		t.Fatal("Bad message!")
		return
	}

	// Accept a deal
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals.. should have atleast one!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+st.ID+"/"+deals[0].Id+"/twitter", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(cmpLoad.Timeline) != 3 {
		t.Fatal("Missing timeline attribute")
		return
	}

	if cmpLoad.Timeline[2].Message != common.DEAL_ACCEPTED {
		t.Fatal("Bad message!")
		return
	}

	// Toggle the campaign to OFF
	updStatus := false
	cmpUpdate := CampaignUpdate{
		Geos:       cmpLoad.Geos,
		Categories: cmpLoad.Categories,
		Status:     &updStatus,
		Budget:     &cmpLoad.Budget,
		Male:       &cmpLoad.Male,
		Female:     &cmpLoad.Female,
		Name:       &cmpLoad.Name,
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(cmpLoad.Timeline) != 4 {
		t.Fatal("Missing timeline attribute")
		return
	}

	if cmpLoad.Timeline[3].Message != common.CAMPAIGN_PAUSED {
		t.Fatal("Bad message!")
		return
	}

	// Toggle the campaign to ON
	updStatus = true
	cmpUpdate = CampaignUpdate{
		Geos:       cmpLoad.Geos,
		Categories: cmpLoad.Categories,
		Status:     &updStatus,
		Budget:     &cmpLoad.Budget,
		Male:       &cmpLoad.Male,
		Female:     &cmpLoad.Female,
		Name:       &cmpLoad.Name,
	}

	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, &cmpUpdate, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(cmpLoad.Timeline) != 5 {
		t.Fatal("Missing timeline attribute")
		return
	}

	if cmpLoad.Timeline[4].Message != common.CAMPAIGN_START {
		t.Fatal("Bad message!")
		return
	}

	var timeline map[string][]*common.Timeline
	r = rst.DoTesting(t, "GET", "/getAdvertiserTimeline/"+cmpLoad.AdvertiserId, nil, &timeline)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(timeline) == 0 {
		t.Fatal("Bad advertiser timeline!")
		return
	}
}

func TestBonus(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "justinbieber",
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Do a deal but DON'T approve yet!
	doDeal(rst, t, inf.ExpID, "2", true)

	// check that the deal is approved
	var newLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &newLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(newLoad.CompletedDeals) == 0 {
		t.Fatal("Unexpected numnber of completed deals!")
		return
	}

	if newLoad.Twitter == nil || len(newLoad.Twitter.LatestTweets) < 3 {
		t.Fatal("No tweets!")
		return
	}

	// Lets try adding a bonus post!
	bonus := Bonus{
		CampaignID:   newLoad.CompletedDeals[0].CampaignId,
		InfluencerID: inf.ExpID,
		PostURL:      newLoad.Twitter.LatestTweets[len(newLoad.Twitter.LatestTweets)-3].PostURL,
	}

	r = rst.DoTesting(t, "POST", "/addBonus", &bonus, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code: %s", r.Value)
		return
	}

	// check that the deal has a bonus post
	var lastLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &lastLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	returnedBonus := lastLoad.CompletedDeals[0].Bonus
	if returnedBonus == nil {
		t.Fatal("No bonus value!")
		return
	}

	if len(returnedBonus.Tweet) == 0 {
		t.Fatal("No twitter bonus value!")
		return
	}

	if returnedBonus.Tweet[0].PostURL != bonus.PostURL {
		t.Fatal("Incorrect post URL value!")
		return
	}

	// Lets try adding a bad bonus post!
	bonus = Bonus{
		CampaignID:   newLoad.CompletedDeals[0].CampaignId,
		InfluencerID: inf.ExpID,
		PostURL:      "www.",
	}

	r = rst.DoTesting(t, "POST", "/addBonus", &bonus, nil)
	if r.Status == 200 {
		t.Fatalf("Bad status code: %s", r.Value)
		return
	}

	bonus = Bonus{
		CampaignID:   "999",
		InfluencerID: inf.ExpID,
		PostURL:      newLoad.Twitter.LatestTweets[0].PostURL,
	}

	r = rst.DoTesting(t, "POST", "/addBonus", &bonus, nil)
	if r.Status == 200 {
		t.Fatalf("Bad status code: %s", r.Value)
		return
	}

	var advDeals []*FeedCell
	r = rst.DoTesting(t, "GET", "/getAdvertiserContentFeed/"+newLoad.CompletedDeals[0].AdvertiserId, nil, &advDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(advDeals) != 2 {
		t.Fatal("Incorrect content feed deals!")
		return
	}

	var bonusDeals int
	for _, d := range advDeals {
		if d.Bonus {
			bonusDeals += 1
		}
	}

	if bonusDeals != 1 {
		t.Fatal("No bonus deals!")
		return
	}
}

func TestProductBudget(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			TwitterId: "cnn",
			Address: &lob.AddressLoad{
				AddressOne: "8 Saint Elias",
				City:       "Trabuco Canyon",
				State:      "CA",
				Country:    "US",
			},
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
		CCLoad:      creditCard,
		SubLoad:     getSubscription(3, 100, true),
	}

	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Create a campaign with NO BUDGET and NO PERKS!
	// Should reject!
	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       0,
		Name:         "Insert cool campaign name",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/campaign", &cmp, &st)
	if r.Status != 400 {
		t.Fatal("Bad status code!")
	}

	// Lets try a campaign with no budget and perks!
	// Should accept
	cmp = common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       0,
		Name:         "Insert cool campaign name",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "haha.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Perks:        &common.Perk{Name: "Nike Air Shoes", Type: 1, Count: 5},
	}

	r = rst.DoTesting(t, "POST", "/campaign", &cmp, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Lets make sure we got the proper budget
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Budget != 0 {
		t.Fatal("Missing timeline attribute")
		return
	}

	var fullStore map[string]budget.Store
	r = rst.DoTesting(t, "GET", "/getStore", nil, &fullStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	store, ok := fullStore[st.ID]
	if !ok {
		t.Fatal("Campaign store not found!")
		return
	}

	if store.Spendable != 0 {
		t.Fatal("Bad spendable")
		return
	}

	if store.Spent != 0 {
		t.Fatal("Bad spent value")
	}

	// approve campaign
	r = rst.DoTesting(t, "GET", "/approveCampaign/"+st.ID, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Budget != 0 {
		t.Fatal("Missing timeline attribute")
		return
	}

	// Accept a deal
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals.. should have atleast one!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+st.ID+"/"+deals[0].Id+"/twitter", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// force approve
	r = rst.DoTesting(t, "GET", "/forceApprove/"+inf.ExpID+"/"+st.ID+"", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Deplete the budget
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	r = rst.DoTesting(t, "GET", "/campaign/"+st.ID+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deal, ok := cmpLoad.Deals[deals[0].Id]
	if !ok {
		t.Fatal("Deal not found!")
		return
	}

	if deal.Completed == 0 {
		t.Fatal("Deal not completed!")
		return
	}

	// Lets make sure it has some stats!
	total := deal.TotalStats()
	if total.Likes == 0 || total.Views == 0 {
		t.Fatal("Missing social media stats for completed post")
	}
}

func TestAudiences(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create an audience

	// Emails should be trimmed!
	members := map[string]bool{
		"jOhn@seanjohn.com": true,
		"blah@fubu.com  ":   true,
	}
	aud := common.Audience{
		Name:    "My test audience",
		Members: members,
	}

	r = rst.DoTesting(t, "POST", "/audience", &aud, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Lets get that audience!
	var audienceStore map[string]common.Audience
	r = rst.DoTesting(t, "GET", "/audience", nil, &audienceStore)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	aud, ok := audienceStore["1"]
	if !ok {
		t.Fatal("Failed to add audience!")
		return
	}

	_, ok = aud.Members["blah@fubu.com"]
	if !ok {
		t.Fatal("Failed to trim email!")
		return
	}

	_, ok = aud.Members["john@seanjohn.com"]
	if !ok {
		t.Fatal("Failed to trim email!")
		return
	}

	// Create a campaign with audience targeting
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:   0.2,
		AgencyID: "2",
	}

	adv.Advertiser.CCLoad = creditCard
	adv.Advertiser.SubLoad = getSubscription(3, 100, true)

	var st Status
	r = rst.DoTesting(t, "POST", "/signUp", adv, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: st.ID,
		Budget:       1000,
		Name:         "Audience campaign",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "http://www.cnn.com?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Audiences:    []string{"1"},
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &status)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}

	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+status.ID, nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if len(cmpLoad.Audiences) != 1 && cmpLoad.Audiences[0] != "1" {
		t.Fatal("Failed to target audience!")
		return
	}

	// Lets create an influencer that SHOULD get a deal because they are in the
	// audience!
	inf := getSignupUserWithEmail("john@seanjohn.com")
	inf.InfluencerLoad = &auth.InfluencerLoad{
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code! %s", r.Value)
		return
	}

	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals(status.ID, deals)
	if len(deals) != 1 {
		t.Fatal("Failed to receive deal!")
		return
	}

	// Lets create an influencer that's NOT in the audience!
	inf = getSignupUserWithEmail("john@notinmyaudience.com")
	inf.InfluencerLoad = &auth.InfluencerLoad{
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "cnn",
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code! %s", r.Value)
		return
	}

	var secondDeals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &secondDeals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	secondDeals = getDeals(status.ID, secondDeals)
	if len(secondDeals) != 0 {
		t.Fatal("Got deal for influencer not in audience!")
		return
	}
}

func TestSubmission(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Create an advertiser that REQUIRES submission!
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:   0.2,
		AgencyID: "2",
		CCLoad:   creditCard,
		SubLoad:  getSubscription(3, 100, true),
	}

	var st Status
	r = rst.DoTesting(t, "POST", "/signUp", adv, &st)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	cmp := common.Campaign{
		Status:             true,
		AdvertiserId:       st.ID,
		Budget:             DEFAULT_BUDGET,
		Name:               "Submission Campaign!",
		Twitter:            true,
		Male:               true,
		Female:             true,
		Link:               "http://www.cnn.com?s=t",
		Task:               "POST THAT DOPE SHIT",
		Tags:               []string{"#mmmm"},
		RequiresSubmission: true,
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &status)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
		return
	}
	cid := status.ID

	if *genData {
		cmp.Name = "Test Submission Cmp"
		r = rst.DoTesting(t, "POST", "/campaign?dbg=1", &cmp, &status)
		if r.Status != 200 {
			t.Fatal("Bad status code!", string(r.Value))
			return
		}
		t.Logf("Advertiser ID: %s, Campaign ID: %s, %s", cmp.AdvertiserId, status.ID)
	}

	// Lets create an influencer that SHOULD get a deal because they are in the
	// audience!
	inf := getSignupUserWithEmail("mayn@mayn123.com")
	inf.InfluencerLoad = &auth.InfluencerLoad{
		InfluencerLoad: influencer.InfluencerLoad{
			Male:      true,
			Geo:       &geo.GeoRecord{},
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatalf("Bad status code! %s", r.Value)
		return
	}

	// get deals for influencer
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	deals = getDeals(cid, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals!")
		return
	}

	tgDeal := deals[0]

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id+"/twitter", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Lets try getting the advertiser to approve the post without there being a submission! Should error!
	r = rst.DoTesting(t, "GET", "/approveSubmission/"+tgDeal.AdvertiserId+"/"+tgDeal.CampaignId+"/"+inf.ExpID, nil, nil)
	if !strings.Contains(string(r.Value), "Deal and submission not found") {
		t.Fatal("Bad value!")
		return
	}

	// Lets now submit the influencer's proposal!
	sub := common.Submission{
		ImageData: []string{postImage},
		ContentURL: []string{"https://www.youtube.com/watch?v=dQw4w9WgXcQ","https://vimeo.com/218549048"},
		Message:   "This is the message this campaign wants #mmmm",
	}

	r = rst.DoTesting(t, "POST", "/submitPost/"+inf.ExpID+"/"+tgDeal.CampaignId, &sub, nil)
	if r.Status != 200 {
		t.Fatal("Bad value!")
		return
	}

	// Lets make sure the submission is there
	var dealGet common.Deal
	r = rst.DoTesting(t, "GET", "/getDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id, nil, &dealGet)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if dealGet.Submission == nil {
		t.Fatal("No submission!")
		return
	}

	if dealGet.Submission.Approved {
		t.Fatal("Submission wrongfully approved!")
		return
	}

	if dealGet.Submission.Message != sub.Message {
		t.Fatal("Bad message in submission!")
		return
	}

	if *genData {
		return
	}

	// Approve the proposal via advertiser (make sure it is now set to approved and there is a submission)
	r = rst.DoTesting(t, "GET", "/approveSubmission/"+tgDeal.AdvertiserId+"/"+tgDeal.CampaignId+"/"+inf.ExpID, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	var dealDone common.Deal
	r = rst.DoTesting(t, "GET", "/getDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id, nil, &dealDone)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if dealDone.Submission == nil {
		t.Fatal("No submission!")
		return
	}

	if !dealDone.Submission.Approved {
		t.Fatal("Submission not approved wtf!")
		return
	}

	if dealDone.Submission.Message != sub.Message {
		t.Fatal("Bad message in submission!")
		return
	}

	if len(dealDone.Submission.ContentURL) != 3 {
		t.Fatal("Bad content URL len!")
		return
	}
}

func TestBilling(t *testing.T) {
	if *genData {
		t.Skip("not needed for generating data")
	}
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	cids := make(map[string]budget.Store)
	addedCids := []string{}

	// Do multiple deals for multiple talent agencies, influencers,
	// and advertisers
	for i := 0; i < 8; i++ {
		ag := getSignupUser()
		ag.TalentAgency = &auth.TalentAgency{
			Fee: 0.1,
		}

		r = rst.DoTesting(t, "POST", "/signUp", &ag, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		inf := getSignupUser()
		inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
			InfluencerLoad: influencer.InfluencerLoad{
				Male:       true,
				Geo:        &geo.GeoRecord{},
				InviteCode: common.GetCodeFromID(ag.ExpID),
				TwitterId:  "breakingnews",
				Address: &lob.AddressLoad{
					AddressOne: "8 Saint Elias",
					City:       "Trabuco Canyon",
					State:      "CA",
					Country:    "US",
				},
			},
		}

		r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		adAg := getSignupUserWithName("Some Ad Agency Name" + strconv.Itoa(i))
		adAg.AdAgency = &auth.AdAgency{Status: true, IsIO: false}
		r = rst.DoTesting(t, "POST", "/signUp", adAg, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		cid := doDeal(rst, t, inf.ExpID, adAg.ExpID, false)

		// Approve the deal
		r = rst.DoTesting(t, "GET", "/forceApprove/"+inf.ExpID+"/"+cid+"", nil, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
			return
		}

		addedCids = append(addedCids, cid)
	}

	// Deplete budgets
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	for _, cid := range addedCids {
		store := budget.Store{}
		r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cid, nil, &store)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		if store.Spent == 0 {
			t.Fatal("Spent not incremented correctly in budget")
		}

		var breakdown map[string]*reporting.Totals
		r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cid+"/10", nil, &breakdown)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
			return
		}
		// Campaign stats and budget should have EVERYTHING (markups + spend)
		total := breakdown["total"]
		if int32(store.Spent) != int32(total.Spent) {
			t.Fatal("Budget and reports do not match!")
		}
		cids[cid] = store
	}

	// Lets see what happens to stores when we bill!
	for cid, oldStore := range cids {
		// LETS RUN BILLING!
		r = rst.DoTesting(t, "GET", "/forceBill/"+cid, nil, nil)
		if r.Status != 200 {
			t.Fatal("Bad status code!", string(r.Value))
		}

		var newStore budget.Store
		r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cid, nil, &newStore)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		if newStore.Spent != 0 {
			t.Fatal("Bad new store values!")
		}

		// Spendable should have been increased!
		if newStore.Spendable < DEFAULT_BUDGET {
			t.Fatal("Bad Spendable values!")
		}

		if newStore.NextBill == 0 || newStore.NextBill == oldStore.NextBill {
			t.Fatal("Bill date did not change!")
		}

		if len(newStore.SpendHistory) != 1 {
			t.Fatal("Bad spend history!")
		}

		val, _ := newStore.SpendHistory[budget.GetSpendHistoryKey()]
		if val == 0 {
			t.Fatal("Bad spend history value!")
		}

		if len(newStore.Charges) != 2 {
			t.Fatal("Wrong number of charges!")
		}
	}
}
