package server

import (
	"fmt"
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
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/internal/reporting"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/platforms/hellosign"
	"github.com/swayops/sway/platforms/lob"
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
	ag.AdAgency = &auth.AdAgency{}

	adv := getSignupUser()
	adv.ParentID = ag.ExpID
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	ag2 := getSignupUser()
	ag2.AdAgency = &auth.AdAgency{}

	adv2 := getSignupUser()
	adv2.ParentID = ag.ExpID
	adv2.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"GET", "/apiKey", nil, 200, nil},
		{"GET", "/getStore", nil, 500, nil},

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
	ag.AdAgency = &auth.AdAgency{}

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
		{"PUT", "/adAgency/" + ag.ExpID, &auth.AdAgency{ID: ag.ExpID, Name: "the rain man", Status: true}, 200, nil},
		{"GET", "/adAgency/" + ag.ExpID, nil, 200, M{"name": "the rain man"}},

		// create a new advertiser as the new agency and signin
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		{"POST", "/signIn", M{"email": adv.Email, "pass": defaultPass}, 200, nil},

		// update the advertiser and check if the update worked
		{"PUT", "/advertiser/" + adv.ExpID, &auth.Advertiser{DspFee: 0.2, ExchangeFee: 0.3}, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: ag.ExpID, DspFee: 0.2, ExchangeFee: 0.3}},

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
			Gender:    "unicorn",
			Geo:       &misc.GeoRecord{},
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
		{"PUT", "/talentAgency/" + ag.ExpID, &auth.TalentAgency{ID: ag.ExpID, Name: "X", Fee: 0.3, Status: true}, 200, nil},
		{"GET", "/talentAgency/" + ag.ExpID, nil, 200, M{"fee": 0.3, "inviteCode": common.GetCodeFromID(ag.ExpID)}},

		// create a new influencer as the new agency and signin
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		// update the influencer and check if the update worked
		{"GET", "/setCategory/" + inf.ExpID + "/business", nil, 200, nil},
		{"GET", "/setPlatform/" + inf.ExpID + "/twitter/" + "SwayOps_com", nil, 200, nil},
		{"POST", "/setGeo/" + inf.ExpID, misc.GeoRecord{City: "hell"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   ag.ExpID,
			"categories": []string{"business"},
			"geo":        M{"city": "hell"},
			"twitter":    M{"id": "SwayOps_com"},
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
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}

	ag := getSignupUser()
	ag.AdAgency = &auth.AdAgency{}

	badAdv := getSignupUser()
	badAdv.Advertiser = &auth.Advertiser{
		DspFee: 2,
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp?autologin=true", adv, 200, misc.StatusOK(adv.ExpID)},

		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: auth.SwayOpsAdAgencyID, DspFee: 0.5}},
		{"PUT", "/advertiser/" + adv.ExpID, &auth.Advertiser{DspFee: 0.2, ExchangeFee: 0.3}, 200, nil},

		// sign in as admin and access the advertiser
		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{DspFee: 0.2}},

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

	if df, ef := getAdvertiserFees(srv.auth, adv.ExpID); df != 0.2 || ef != 0.3 {
		t.Fatal("getAdvertiserFees failed", df, ef)
	}

}

func TestNewInfluencer(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Gender:      "unicorn",
			Geo:         &misc.GeoRecord{},
			TwitterId:   "justinbieber",
			InstagramId: "justinbieber",
		},
	}

	badInf := getSignupUser()
	badInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Gender: "purple",
			Geo:    &misc.GeoRecord{},
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		// update
		{"GET", "/setCategory/" + inf.ExpID + "/business", nil, 200, nil},
		{"GET", "/setPlatform/" + inf.ExpID + "/twitter/" + "SwayOps_com", nil, 200, nil},
		{"POST", "/setGeo/" + inf.ExpID, misc.GeoRecord{City: "hell"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"geo":        M{"city": "hell"},
			"categories": []string{"business"},
			"twitter":    M{"id": "SwayOps_com"},
		}},

		// Add a social media platofrm
		{"GET", "/setPlatform/" + inf.ExpID + "/facebook/" + "justinbieber", nil, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"geo":        M{"city": "hell"},
			"categories": []string{"business"},
			"twitter":    M{"id": "SwayOps_com"},
			"facebook":   M{"id": "justinbieber"},
		}},

		// try to load it as a different user
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 401, nil},

		{"POST", "/signIn", adminReq, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, nil},

		{"POST", "/signUp", badInf, 400, nil},
	} {
		tr.Run(t, rst)
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
			Gender:     "unicorn",
			Geo:        &misc.GeoRecord{},
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
	ag.AdAgency = &auth.AdAgency{}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}
	cmp := common.Campaign{
		AdvertiserId: adv.ExpID,
		Budget:       10.5,
		Name:         "The Day Walker",
		Instagram:    true,
		Gender:       "mf",
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Whitelist: &common.TargetList{
			Instagram: []string{"someguy"},
		},
	}
	cmpUpdate1 := `{"name":"Blade V","budget":10.5,"status":true,"hashtags":["mmmm"],"link":"blade.org","gender":"f","instagram":true, "whitelist":{"instagram": ["justinbieber"]}}`
	cmpUpdate2 := `{"advertiserId": "` + adv.ExpID + `", "name":"Blade VI?","budget":10.5,"status":true,"hashtags":["mmmm"],"link":"blade.org","gender":"f","instagram":true}`
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

		{"POST", "/campaign", &cmp, 200, nil},
		{"PUT", "/campaign/1", cmpUpdate1, 200, nil},
		{"GET", "/campaign/1", nil, 200, M{
			"name":         "Blade V",
			"agencyId":     ag.ExpID,
			"advertiserId": adv.ExpID,
			"whitelist":    M{"instagram": []string{"justinbieber"}},
		}},

		// access the campaign with the agency
		{"POST", "/signIn", M{"email": ag.Email, "pass": defaultPass}, 200, nil},
		{"PUT", "/campaign/1", cmpUpdate2, 200, nil},
		{"GET", "/campaign/1", nil, 200, M{"name": "Blade VI?"}},

		// sign in as a different ad agency and try to access the campaign
		{"POST", "/signIn", adminAdAgencyReq, 200, nil},
		{"GET", "/campaign/1", nil, 401, nil},

		// try to create a campaign with a bad advertiser id
		{"POST", "/campaign", &badAdvId, 400, nil},
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
			Gender:     "m",
			Geo:        &misc.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "breakingnews",
		},
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       5000.5,
		Name:         "The Day Walker",
		Twitter:      true,
		Gender:       "mf",
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Whitelist: &common.TargetList{
			Twitter: []string{"BreakingNews", "CNN"},
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

		// sign in as admin again
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},

		// create new advertiser and sign in
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		// create a new campaign
		{"POST", "/campaign", &cmp, 200, nil},

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

	// Sign up as a second influencer and do a deal! Need to see
	// cumulative stats
	newInf := getSignupUser()
	newInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Gender:     "m",
			Geo:        &misc.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "CNN",
			Categories: []string{"business"},
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
	var pendingInf []influencer.Influencer
	r := rst.DoTesting(t, "GET", "/getIncompleteInfluencers", nil, &pendingInf)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	for _, i := range pendingInf {
		if i.Id == newInf.ExpID {
			t.Error("Unexpected influencer in incomplete list!")
		}
		if i.Id == inf.ExpID {
			t.Error("Unexpected influencer in incomplete list!")
		}
	}

	// Check reporting for just the second influencer
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+newInf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	var breakdownB map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+newInf.ExpID+"/10", nil, &breakdownB)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	checkReporting(t, breakdownB, 0, load.CompletedDeals[0], true)

	// Verify combined reporting because campaign reports will include both
	var newStore budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/2", nil, &newStore)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	var breakdownA map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+inf.ExpID+"/10", nil, &breakdownA)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	totalA := breakdownA["total"]
	totalB := breakdownB["total"]

	totalShares := totalA.Shares + totalB.Shares
	totalLikes := totalA.Likes + totalB.Likes
	totalSpend := totalA.Spent + totalB.Spent

	if agencyCut := (newStore.Spent - totalSpend) / newStore.Spent; agencyCut > 0.12 || agencyCut < 0.08 {
		t.Error("Combined spend does not match budget db!")
	}

	var cmpBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignStats/2/10", nil, &cmpBreakdown)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	totalCmp := cmpBreakdown["total"]
	if delta := totalCmp.Spent - newStore.Spent; delta > 0.25 || delta < -0.25 {
		t.Error("Combined spend does not match campaign report!")
	}

	if totalCmp.Shares != totalShares {
		t.Error("Combined shares do not match campaign report!")
	}

	if totalCmp.Likes != totalLikes {
		t.Error("Combined likes do not match campaign report!")
	}

	if totalCmp.Influencers != 2 {
		t.Error("Influencer count incorrect!")
	}

	var cmpInfBreakdown map[string]*reporting.Totals
	r = rst.DoTesting(t, "GET", "/getCampaignInfluencerStats/2/"+newInf.ExpID+"/10", nil, &cmpInfBreakdown)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	totalCmpInf := cmpInfBreakdown["total"]
	if totalCmpInf.Likes != totalB.Likes {
		t.Error("Combined likes do not match campaign report!")
	}

	if totalCmpInf.Shares != totalB.Shares {
		t.Error("Combined shares do not match campaign report!")
	}
}

func verifyDeal(t *testing.T, cmpId, infId, agId string, rst *resty.Client, skipReporting bool) {
	var oldStore budget.Store
	r := rst.DoTesting(t, "GET", "/getBudgetInfo/"+cmpId, nil, &oldStore)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	checkStore(t, &oldStore, nil)

	// deplete budget according to the payout
	r = rst.DoTesting(t, "GET", "/forceDeplete", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// check for money in completed deals (fees and inf)
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+infId, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	if len(load.CompletedDeals) != 1 {
		t.Error("Could not find completed deal!")
	}

	doneDeal := load.CompletedDeals[0]
	checkDeal(t, doneDeal, &load, agId, cmpId)

	var newStore budget.Store
	r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cmpId, nil, &newStore)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if newStore.Spent == 0 {
		t.Error("Spent not incremented correctly in budget")
	}
	checkStore(t, &newStore, &oldStore)

	// check for money in campaign deals (fees and inf)
	var cmpLoad common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/"+cmpId+"?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	doneDeal = cmpLoad.Deals[load.CompletedDeals[0].Id]
	if doneDeal == nil {
		t.Error("Cannot find done deal in campaign!")
	}

	checkDeal(t, doneDeal, &load, agId, cmpId)

	if !skipReporting {
		// check get campaign stats
		var breakdown map[string]*reporting.Totals
		r = rst.DoTesting(t, "GET", "/getCampaignStats/"+cmpId+"/10", nil, &breakdown)
		if r.Status != 200 {
			t.Error("Bad status code!")
		}

		checkReporting(t, breakdown, newStore.Spent, doneDeal, false)

		// check get influencer stats
		r = rst.DoTesting(t, "GET", "/getInfluencerStats/"+infId+"/10", nil, &breakdown)
		if r.Status != 200 {
			t.Error("Bad status code!")
		}
		checkReporting(t, breakdown, newStore.Spent, doneDeal, false)
	}
}

func checkStore(t *testing.T, store, compareStore *budget.Store) {
	if dspFee := store.DspFee / (store.Spendable + store.Spent + store.ExchangeFee + store.DspFee); dspFee > 0.21 || dspFee < 0.19 {
		t.Error("Unexpected DSP fee", dspFee)
	}

	if exchangeFee := store.ExchangeFee / (store.Spendable + store.Spent + store.ExchangeFee + store.DspFee); exchangeFee > 0.11 || exchangeFee < 0.09 {
		t.Error("Unexpected Exchange fee", exchangeFee)
	}

	if compareStore != nil {
		oldV := store.Spent + store.Spendable
		newV := compareStore.Spendable + compareStore.Spent
		if newV != oldV {
			t.Error("Spendable and spent not synchronized!")
		}
	}
}

func checkDeal(t *testing.T, doneDeal *common.Deal, load *influencer.Influencer, agId, campaignId string) {
	if doneDeal.CampaignId != campaignId {
		t.Error("Campaign ID not assigned!")
	}

	if doneDeal.Assigned == 0 || doneDeal.Completed == 0 {
		t.Error("Deal timestamps missing!")
	}

	if doneDeal.InfluencerId != load.Id {
		t.Error("Deal ID missing!")
	}

	var m *common.Payout
	if m = doneDeal.GetPayout(0); m != nil {
		if m.AgencyId != agId {
			t.Error("Payout to wrong talent agency!")
		}
		if m.Influencer == 0 {
			t.Error("No influencer payout!")

		}
		if m.Agency == 0 {
			t.Error("No agency payout!")
		}
		// Should be 10% as stated earlier on talent agency initialization
		if agencyFee := m.Agency / (m.Agency + m.Influencer); agencyFee > 0.11 || agencyFee < 0.09 {
			t.Error("Unexpected agency fee", agencyFee)
		}
	} else {
		t.Error("Could not find completed deal payout!")
	}

	if load.PendingPayout != m.Influencer {
		t.Error("Unexpected pending payout!")
	}

	if m = doneDeal.GetPayout(1); m != nil {
		t.Error("How the hell are you getting payouts from last month?")
	}
}

func checkReporting(t *testing.T, breakdown map[string]*reporting.Totals, spend float64, doneDeal *common.Deal, skipSpend bool) {
	report := breakdown["total"]
	dayTotal := breakdown[reporting.GetDate()]
	rt := int32(doneDeal.Tweet.Retweets)

	if rt != dayTotal.Shares || rt != report.Shares {
		t.Error("Shares do not match!")
	}

	likes := int32(doneDeal.Tweet.Favorites)
	if likes != dayTotal.Likes || likes != report.Likes {
		t.Error("Likes do not match!")
	}

	if !skipSpend {
		if report.Spent != spend {
			// If spend does not match (i.e. we just pulled influencer stats which doesnt include agency spend)
			if talentFee := (spend - report.Spent) / report.Spent; talentFee > 0.12 || talentFee < 0.08 {
				t.Error("Spend values do not match!")
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

	inf := getSignupUserWithEmail("shahzilsway@gmail.com") //throw away email
	inf.InfluencerLoad = &auth.InfluencerLoad{             // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Gender:     "m",
			Geo:        &misc.GeoRecord{},
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
		t.Error("Bad status code for initial check!")
	}

	if load.SignatureId != "" || load.HasSigned {
		t.Error("Unexpected signing!")
	}

	r = rst.DoTesting(t, "GET", "/emailTaxForm/"+inf.ExpID, nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code for email!")
	}

	var uload influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &uload)
	if r.Status != 200 {
		t.Error("Bad status code second influencer check!")
	}

	if uload.SignatureId == "" {
		t.Error("No signature id assigned!")
	}

	if uload.RequestedTax == 0 {
		t.Error("No tax request timestamp!")
	}

	val, err := hellosign.HasSigned(inf.ExpID, uload.SignatureId)
	if val || err != nil {
		t.Error("Error getting signed value!")
	}

	// Cleanup
	time.Sleep(5 * time.Second)
	if r, err := hellosign.Cancel(uload.SignatureId); err != nil || r != 200 {
		t.Error("Hellosign cancel error")
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
			Gender:     "m",
			Geo:        &misc.GeoRecord{},
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
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       100.5,
		Name:         "The Day Walker",
		Twitter:      true,
		Gender:       "mf",
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Whitelist: &common.TargetList{
			Twitter: []string{"BreakingNews", "CNN"},
		},
		Perks: &common.Perk{Name: "Nike Air Shoes", Category: "product", Count: 5},
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
	r := rst.DoTesting(t, "GET", "/campaign/3?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(cmpLoad.Deals) != cmp.Perks.Count {
		t.Error("Unexpected number of deals!")
	}

	if cmp.IsValid() {
		t.Error("Campaign should not be valid!")
	}

	if cmp.Approved {
		t.Error("Campaign should not be approved!")
	}

	// make sure influencer getting no deals since the campaign is still pending
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	deals = getDeals("3", deals)
	if len(deals) > 0 {
		t.Error("Unexpected number of deals!")
	}

	// check admin endpoints for campaign approval
	var cmps []*common.Campaign
	r = rst.DoTesting(t, "GET", "/getPendingCampaigns", nil, &cmps)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(cmps) != 1 {
		t.Error("Unexpected number of pending campaigns")
	}

	if cmps[0].Id != "3" {
		t.Error("Unexpected campaign id!")
	}

	if cmps[0].Approved {
		t.Error("Unexpected approval value!")
	}

	// approve campaign
	r = rst.DoTesting(t, "GET", "/approveCampaign/3", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// verify pending campaigns is empty!
	r = rst.DoTesting(t, "GET", "/getPendingCampaigns", nil, &cmps)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(cmps) != 0 {
		t.Error("Pending campaigns shouldnt be here!")
	}

	// make sure approved value is correct!
	r = rst.DoTesting(t, "GET", "/campaign/3", nil, &cmpLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if !cmpLoad.Approved {
		t.Error("Admin approval didnt work!")
	}

	// get deals for influencer
	r = rst.DoTesting(t, "GET", "/getDeals/"+inf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	deals = getDeals("3", deals)
	if len(deals) == 0 {
		t.Error("Unexpected number of deals!")
	}

	for _, d := range deals {
		if d.CampaignImage == "" {
			t.Error("Missing image url!")
		}
	}

	tgDeal := deals[0]
	if tgDeal.CampaignId != "3" {
		t.Error("Unexpected campaign id!")
	}

	if tgDeal.Perk == nil {
		t.Error("Should have a perk attached!")
	}

	if tgDeal.Perk.Count != 1 {
		t.Error("Incorrect reporting of perk count")
	}

	if tgDeal.Perk.InfId != "" || tgDeal.Perk.Status {
		t.Error("Incorrect perk values set!")
	}

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id+"/twitter", nil, &deals)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// check campaign perk status and count (make sure both were updated)
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(load.ActiveDeals) != 1 {
		t.Error("Unexpected number of active deals!")
	}

	tgDeal = load.ActiveDeals[0]
	if tgDeal.Perk == nil {
		t.Error("No perk assigned!")
	}

	if tgDeal.Perk.Status {
		t.Error("Unexpected perk status!")
	}

	if tgDeal.Perk.InfId != inf.ExpID {
		t.Error("Incorrect inf id set for perk!")
	}

	if tgDeal.Perk.Address == nil {
		t.Error("No address set for perk!")
	}

	if tgDeal.CampaignId != "3" {
		t.Error("Unexpected campaign id for deal!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/3?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if cmpLoad.Perks == nil {
		t.Error("Campaign has no perks man!")
	}

	if cmpLoad.Perks.Count != 4 {
		t.Error("Campaign perk count did not decrement!")
	}

	cmpDeal, ok := cmpLoad.Deals[tgDeal.Id]
	if !ok || cmpDeal.Perk == nil {
		t.Error("Unexpected campaign deal value!")
	}

	if cmpDeal.Perk.InfId != inf.ExpID {
		t.Error("Influencer ID not assigned to campaign deal")
	}

	if cmpDeal.Perk.Address == nil {
		t.Error("Unexpected deal address")
	}

	if cmpDeal.Perk.Status {
		t.Error("Campaign deal should not be approved yet!")
	}

	// get pending perk sendouts for admin
	var pendingPerks map[string][]*common.Perk
	r = rst.DoTesting(t, "GET", "/getPendingPerks", nil, &pendingPerks)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(pendingPerks) != 1 {
		t.Error("Unexpected number of perks.. should have 1!")
	}

	pk, ok := pendingPerks["3"]
	if !ok {
		t.Error("Perk request not found")
	}

	if len(pk) != 1 {
		t.Error("Unexpected number of perks")
	}

	if pk[0].Status {
		t.Error("Incorrect perk status value!")
	}

	if pk[0].Address == nil {
		t.Error("No address for perk!")
	}

	// approve sendout
	r = rst.DoTesting(t, "GET", "/approvePerk/"+inf.ExpID+"/3", nil, &pendingPerks)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// make sure get pending perk doesnt have that perk request now
	var emptyPerks map[string][]*common.Perk
	r = rst.DoTesting(t, "GET", "/getPendingPerks", nil, &emptyPerks)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(emptyPerks) != 0 {
		t.Error("Pending perk still leftover!")
	}

	// make sure status is now true on campaign and influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	tgDeal = load.ActiveDeals[0]
	if tgDeal.Perk == nil {
		t.Error("No perk assigned!")
	}

	if !tgDeal.Perk.Status {
		t.Error("Deal perk status should be true!")
	}

	r = rst.DoTesting(t, "GET", "/campaign/3?deals=true", nil, &cmpLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	cmpDeal, ok = cmpLoad.Deals[tgDeal.Id]
	if !ok || cmpDeal.Perk == nil {
		t.Error("Unexpected campaign deal value!")
	}

	if !cmpDeal.Perk.Status {
		t.Error("Campaign deal should be approved now!")
	}

	// force approve
	r = rst.DoTesting(t, "GET", "/forceApprove/"+inf.ExpID+"/3", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	// verify deal
	verifyDeal(t, "3", inf.ExpID, ag.ExpID, rst, false)
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

func TestScraps(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Create 15 scraps
	for i := 0; i < 5; i++ {
		sc := &influencer.Scrap{
			EmailAddress: strconv.Itoa(i) + "bs@bs.com",
			TwitterID:    "cnn",
		}

		r := rst.DoTesting(t, "POST", "/scrap", &sc, nil)
		if r.Status != 200 {
			t.Error("Bad status code!")
		}

	}

	var incomplete []*influencer.Scrap
	r = rst.DoTesting(t, "GET", "/getIncompleteScraps", nil, &incomplete)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Should have 5 incomplete scraps
	if len(incomplete) != 5 {
		t.Error("Unexpected number of incomplete deals!")
	}

	// Fix one
	sc := &influencer.Scrap{
		Categories: []string{"bUsiness"},
		Gender:     "f",
	}
	r = rst.DoTesting(t, "PUT", "/scrap/3", &sc, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	var newSc influencer.Scrap
	r = rst.DoTesting(t, "GET", "/scrap/3", nil, &newSc)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(newSc.Categories) == 0 || newSc.Gender != "f" {
		t.Error("Unexpected scrap data!")
	}

	// Should have 4 scraps now that we've fixed one
	r = rst.DoTesting(t, "GET", "/getIncompleteScraps", nil, &incomplete)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(incomplete) != 4 {
		t.Error("Unexpected number of scraps!")
	}

	// Create a few campaigns
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	for i := 1; i < 8; i++ {
		cmp := common.Campaign{
			Status:       true,
			AdvertiserId: "4",
			Budget:       float64(i * 10),
			Name:         "The Day Walker",
			Twitter:      true,
			Gender:       "mf",
			Link:         "blade.org",
			Tags:         []string{"#mmmm"},
			Whitelist: &common.TargetList{
				Twitter: []string{"BreakingNews", "CNN"},
			},
		}
		r := rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
		if r.Status != 200 {
			t.Error("Bad status code!")
		}
	}

	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Check to see if this dude got an email!
	r = rst.DoTesting(t, "GET", "/scrap/3", nil, &newSc)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(newSc.SentEmails) != 1 {
		t.Error("Failed to send email!")
		return
	}

	if len(newSc.SentEmails[0].Campaigns) != 5 {
		t.Error("Wrong number of campaigns emailed!")
	}

	// Try emailing again.. it should skip all influencers
	// since we just emailed!
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	r = rst.DoTesting(t, "GET", "/scrap/3", nil, &newSc)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if len(newSc.SentEmails) != 1 {
		t.Error("Wrongfully sent an email!")
	}

	// Sign up as an influencer now with the same email
	// Should delete the old scrap since emails will match!

	// Create an influencer with the same email
	inf := getSignupUserWithEmail(newSc.EmailAddress)
	inf.InfluencerLoad = &auth.InfluencerLoad{
		InfluencerLoad: influencer.InfluencerLoad{
			Gender:    "m",
			Geo:       &misc.GeoRecord{},
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	// Force emails
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// This scrap should no longer be here since we
	// signed up with it's email
	// ForceEmail should have deleted it!
	r = rst.DoTesting(t, "GET", "/scrap/3", nil, &newSc)
	if r.Status != 400 {
		t.Error("WTF Scrap still exists!!")
	}

	// Make sure other scraps still there!
	r = rst.DoTesting(t, "GET", "/scrap/4", nil, &newSc)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if newSc.EmailAddress == "" {
		t.Error("Empty scrap!")
	}

}

func TestInfluencerEmail(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Create an influencer
	inf := getSignupUserWithEmail("shahzilabid@gmail.com")
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Gender:    "m",
			Geo:       &misc.GeoRecord{},
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Lets try forcing an email run
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Influencer's email ping setting is off.. they should have
	// no email sent!
	var load influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if load.DealPing || load.LastEmail != 0 {
		t.Error("Unexpected email values for influencer")
	}

	// Lets set this influencer to receive emails now!
	r = rst.DoTesting(t, "POST", "/setReminder/"+inf.ExpID+"/true", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if !load.DealPing {
		t.Error("Deal ping did not toggle!")
	}

	// Create some campaigns so the influencer gets an email!
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}
	for i := 1; i < 8; i++ {
		cmp := common.Campaign{
			Status:       true,
			AdvertiserId: "4",
			Budget:       float64(i * 10),
			Name:         "The Day Walker " + strconv.Itoa(i),
			Twitter:      true,
			Gender:       "mf",
			Link:         "blade.org",
			Task:         "POST THAT DOPE SHIT " + strconv.Itoa(i) + " TIMES!",
			Tags:         []string{"#mmmm"},
			Whitelist: &common.TargetList{
				Twitter: []string{"BreakingNews", "CNN"},
			},
		}
		r := rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
		if r.Status != 200 {
			t.Error("Bad status code!")
		}
	}

	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Should have an email now!
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if load.LastEmail == 0 {
		t.Error("No email was sent wth!")
	}

	// Lets force email again and make sure a new email doesnt get sent
	// since we haven't reached threshold yet
	r = rst.DoTesting(t, "GET", "/forceEmail", nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	var newLoad influencer.Influencer
	r = rst.DoTesting(t, "GET", "/influencer/"+inf.ExpID, nil, &newLoad)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if load.LastEmail != newLoad.LastEmail {
		t.Error("A new email was sent.. HOW?!")
	}
}

func TestImages(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Sign in as admin
	r := rst.DoTesting(t, "POST", "/signIn", &adminReq, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Create the campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.1,
	}
	r = rst.DoTesting(t, "POST", "/signUp", adv, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	cmp := common.Campaign{
		AdvertiserId: adv.ExpID,
		Budget:       10.5,
		Name:         "The Day Walker",
		Instagram:    true,
		Gender:       "mf",
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Whitelist: &common.TargetList{
			Instagram: []string{"someguy"},
		},
	}

	r = rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Make sure the default image url was set
	var load common.Campaign
	r = rst.DoTesting(t, "GET", "/campaign/1", nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if !strings.Contains(load.ImageURL, "/images/campaign/default") {
		t.Error("Incorrect default image set!")
	}

	// HTTPTest doesn't use the port from sway config.. so lets account for that!
	parts := strings.Split(load.ImageURL, "8080") // DIRTY HACK
	if len(parts) != 2 {
		t.Error("WTF MATE?")
		return
	}

	r = rst.DoTesting(t, "GET", parts[1], nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Lets try hitting an image which doesnt exist
	r = rst.DoTesting(t, "GET", "/images/campaign/default_dne.jpg", nil, nil)
	if r.Status == 200 {
		t.Error("Bad status code!")
	}

	// Try uploading a bad image
	r = rst.DoTesting(t, "POST", "/uploadImage/1/campaign", smallImage, nil)
	if r.Status != 400 {
		t.Error("Bad status code!")
	}

	// Upload correct image
	r = rst.DoTesting(t, "POST", "/uploadImage/1/campaign", goodImage, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Make sure image url now correct
	r = rst.DoTesting(t, "GET", "/campaign/1", nil, &load)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	if load.ImageURL == "" || strings.Contains(load.ImageURL, "default") {
		t.Error("Incorrect image url!")
	}

	// make sure image url works
	// HTTPTest doesn't use the port from sway config.. so lets account for that!
	parts = strings.Split(load.ImageURL, "8080") // DIRTY HACK
	if len(parts) != 2 {
		t.Error("WTF MATE?")
		return
	}

	r = rst.DoTesting(t, "GET", parts[1], nil, nil)
	if r.Status != 200 {
		t.Error("Bad status code!")
	}

	// Remove saved image (and indirectly check if it exists)!
	err := os.Remove(".." + parts[1])
	if err != nil {
		t.Error("File does not exist!", ".."+parts[1])
	}
}
