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
	"github.com/swayops/sway/internal/geo"
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
		{"PUT", "/adAgency/" + ag.ExpID, &auth.User{AdAgency: &auth.AdAgency{ID: ag.ExpID, Name: "the rain man", Status: true}}, 200, nil},
		{"GET", "/adAgency/" + ag.ExpID, nil, 200, M{"name": "the rain man"}},

		// create a new advertiser as the new agency and signin
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		{"POST", "/signIn", M{"email": adv.Email, "pass": defaultPass}, 200, nil},

		// update the advertiser and check if the update worked
		{"PUT", "/advertiser/" + adv.ExpID, &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.2}}, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: ag.ExpID, DspFee: 0.2, ExchangeFee: 0.2}},

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
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "SwayOps_com"}, 200, nil},
		{"PUT", "/setCategories/" + inf.ExpID, M{"categories": []string{"business"}}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   ag.ExpID,
			"categories": []string{"business"},
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
		DspFee: 0.5,
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
		{"PUT", "/advertiser/" + adv.ExpID, &auth.User{Advertiser: &auth.Advertiser{DspFee: 0.2}}, 200, nil},

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
		{"PUT", "/setCategories/" + inf.ExpID, M{"categories": []string{"business"}}, 200, nil},
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "SwayOps_com"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"categories": []string{"business"},
			"twitter":    M{"id": "SwayOps_com"},
		}},

		// Add a social media platofrm
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "SwayOps_com", "facebook": "justinbieber"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"categories": []string{"business"},
			"twitter":    M{"id": "SwayOps_com"},
			"facebook":   M{"id": "justinbieber"},
		}},

		// change their password
		{"PUT", "/influencer/" + inf.ExpID, M{"twitter": "SwayOps_com", "facebook": "justinbieber", "oldPass": defaultPass, "pass":"newPassword", "pass2":"newPassword"}, 200, nil},
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

	if len(cats) == 0 {
		t.Fatal("No categories!")
		return
	}

	for _, i := range cats {
		if i.Category == "business" && i.Influencers != 2 && i.Reach == 0 {
			t.Fatal("Unexpected category count!")
			return
		}
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
	ag.AdAgency = &auth.AdAgency{}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee: 0.5,
	}
	cmp := common.Campaign{
		AdvertiserId: adv.ExpID,
		Budget:       150,
		Name:         "The Day Walker",
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

		{"POST", "/campaign", &cmp, 200, nil},
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
			Male:       true,
			Geo:        &geo.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "breakingnews",
		},
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee: 0.2,
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       5000.5,
		Name:         "The Day Walker",
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
			Male:       true,
			Geo:        &geo.GeoRecord{},
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
	var pendingInf []IncompleteInfluencer
	r := rst.DoTesting(t, "GET", "/getIncompleteInfluencers", nil, &pendingInf)
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
		if advDeal.Views == 0 || advDeal.URL == "" || advDeal.Caption == "" {
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
		Name:         "The Day Walker",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "http://www.blank.org?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign", &cmpBlacklist, &status)
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
	if store != nil && compareStore != nil {
		if store.DspFee != compareStore.DspFee && store.ExchangeFee != compareStore.ExchangeFee {
			t.Fatal("Fees changed!")
		}
	}

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
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       150.5,
		Name:         "The Day Walker",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Perks:        &common.Perk{Name: "Nike Air Shoes", Category: "product", Count: 5},
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
		t.Fatal("Bad status code!", string(r.Value))
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

	if cmp.Approved > 0 {
		t.Fatal("Campaign should not be approved!")
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
	r = rst.DoTesting(t, "GET", "/campaign/4", nil, &cmpLoad)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if cmpLoad.Approved == 0 {
		t.Fatal("Admin approval didnt work!")
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

	// pick up deal for influencer
	r = rst.DoTesting(t, "GET", "/assignDeal/"+inf.ExpID+"/"+tgDeal.CampaignId+"/"+tgDeal.Id+"/twitter", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
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

	if cmpLoad.Perks.Count != 4 {
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
	var pendingPerks map[string][]*common.Perk
	r = rst.DoTesting(t, "GET", "/getPendingPerks", nil, &pendingPerks)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if len(pendingPerks) != 1 {
		t.Fatal("Unexpected number of perks.. should have 1!")
	}

	pk, ok := pendingPerks["4"]
	if !ok {
		t.Fatal("Perk request not found")
	}

	if len(pk) != 1 {
		t.Fatal("Unexpected number of perks")
	}

	if pk[0].Status {
		t.Fatal("Incorrect perk status value!")
	}

	if pk[0].Address == nil {
		t.Fatal("No address for perk!")
	}

	var emptyPerks map[string][]*common.Perk

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
		t.Fatal("Bad status code!")
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
	updLoad := &InfluencerUpdate{DealPing: false, TwitterId: "cnn", Gender: "m"}
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
	updLoad = &InfluencerUpdate{DealPing: true, TwitterId: "cnn", Gender: "m"}
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
			Name:         "The Day Walker " + strconv.Itoa(i),
			Twitter:      true,
			Male:         true,
			Female:       true,
			Link:         "blade.org",
			Task:         "POST THAT DOPE SHIT " + strconv.Itoa(i) + " TIMES!",
			Tags:         []string{"#mmmm"},
		}
		r := rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
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

	r = rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
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

	// HTTPTest doesn't use the port from sway config.. so lets account for that!
	parts := strings.Split(load.ImageURL, "8080") // DIRTY HACK
	if len(parts) != 2 {
		t.Fatal("WTF MATE?")
		return
	}

	r = rst.DoTesting(t, "GET", parts[1], nil, nil)
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

	// make sure image url works
	// HTTPTest doesn't use the port from sway config.. so lets account for that!
	parts = strings.Split(load.ImageURL, "8080") // DIRTY HACK
	if len(parts) != 2 {
		t.Fatal("WTF MATE?")
		return
	}

	r = rst.DoTesting(t, "GET", parts[1], nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Remove saved image (and indirectly check if it exists)!
	err := os.Remove("./" + parts[1])
	if err != nil {
		t.Fatal("File does not exist!", ".."+parts[1])
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

	if load.State != "NY" || load.Country != "US" {
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
		Name:         "The Day Walker",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "blade.org",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
		Geos:         fakeGeo,
	}

	r = rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
	if r.Status == 200 {
		t.Fatal("Bad status code!")
	}

	// Lets try creating a campaign with a non-US country
	// trying to do state targeting!
	fakeGeo = []*geo.GeoRecord{
		&geo.GeoRecord{State: "ON", Country: "GB"},
	}
	cmp.Geos = fakeGeo
	r = rst.DoTesting(t, "POST", "/campaign", &cmp, nil)
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
	r = rst.DoTesting(t, "POST", "/campaign", &cmp, &st)
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

	// Update campaign to have bad geo.. Should reject!
	cmpUpdateBad := `{"geos": [{"state": "TX", "country": "GB"}], "name":"Blade V","budget":10,"status":true,"tags":["mmmm"],"male":true,"female":true,"twitter":true}`
	r = rst.DoTesting(t, "PUT", "/campaign/"+st.ID, cmpUpdateBad, nil)
	if r.Status == 200 {
		t.Fatal("Unexpected status code!")
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
	ukInf := getSignupUser()
	ukInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			IP:        "131.228.17.26", // London
			TwitterId: "cnn",
		},
	}
	r = rst.DoTesting(t, "POST", "/signUp", &ukInf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	// Influencer should get a deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+ukInf.ExpID+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	deals = getDeals(st.ID, deals)
	if len(deals) == 0 {
		t.Fatal("Unexpected number of deals!")
	}
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

func doDeal(rst *resty.Client, t *testing.T, infId, agId string, approve bool) (cid string) {
	// Create a campaign
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:   0.2,
		AgencyID: agId,
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
		Budget:       1000,
		Name:         "The Day Walker",
		Twitter:      true,
		Male:         true,
		Female:       true,
		Link:         "http://www.blank.org?s=t",
		Task:         "POST THAT DOPE SHIT",
		Tags:         []string{"#mmmm"},
	}

	var status Status
	r = rst.DoTesting(t, "POST", "/campaign", &cmp, &status)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}
	cid = status.ID

	// get deals for influencer
	var deals []*common.Deal
	r = rst.DoTesting(t, "GET", "/getDeals/"+infId+"/0/0", nil, &deals)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
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

func TestClicks(t *testing.T) {
	rst := getClient()
	defer putClient(rst)

	// Make sure click endpoint accessible without signing in
	r := rst.DoTesting(t, "GET", "/click/1/1/1", nil, nil)
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
			TwitterId: "justinbieber",
		},
	}

	r = rst.DoTesting(t, "POST", "/signUp", &inf, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	// Do a deal but DON'T approve yet!
	doDeal(rst, t, inf.ExpID, "2", false)

	// Influencer has assigned deals.. lets try clicking!
	// It shouldn't allow it!
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

	if !strings.Contains(cmpLoad.Link, "blank.org") {
		t.Fatal("Shortening of the URL did not work!")
		return
	}

	if !strings.Contains(load.ActiveDeals[0].ShortenedLink, "goo.gl") {
		t.Fatal("Unexpected shortened link")
		return
	}

	// Try faking a click for an active deal.. shouldn't work but should redirect!
	r = rst.DoTesting(t, "GET", "/click/"+inf.ExpID+"/"+cid+"/"+load.ActiveDeals[0].Id, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(r.URL, "blank.org") {
		t.Fatal("Incorrect redirect")
		return
	}

	// Make sure there are no clicks
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

	if !strings.Contains(newLoad.CompletedDeals[0].ShortenedLink, "goo.gl") {
		t.Fatal("Bad shortened link")
		return
	}

	// Try a real click
	// Can't hit the shortened link since it will redirect to localhost! NO MAS!
	r = rst.DoTesting(t, "GET", "/click/"+inf.ExpID+"/"+cid+"/"+newLoad.CompletedDeals[0].Id, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
	}

	if !strings.Contains(r.URL, "blank.org") {
		t.Fatal("Incorrect redirect")
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

	// Try clicking again.. should fail because of unique check!
	r = rst.DoTesting(t, "GET", "/click/"+inf.ExpID+"/"+cid+"/"+newLoad.CompletedDeals[0].Id, nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!")
		return
	}

	if !strings.Contains(r.URL, "blank.org") {
		t.Fatal("Incorrect redirect")
		return
	}

	// Make sure nothing increments since this uuid just had a click
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

	decreaseCid := ""

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
		adAg.AdAgency = &auth.AdAgency{Status: true}
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

		if i == 6 {
			oldStore := budget.Store{}
			r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cid, nil, &oldStore)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
			}

			var oldLoad common.Campaign
			r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &oldLoad)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
			}

			// For the second last campaign lets increase the budget!
			trueVal := true
			budgetVal := float64(5000)
			name := "The day wlker"
			upd := &CampaignUpdate{Status: &trueVal, Male: &trueVal, Female: &trueVal, Budget: &budgetVal, Name: &name}
			r = rst.DoTesting(t, "PUT", "/campaign/"+cid, upd, nil)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
				return
			}

			var load common.Campaign
			r = rst.DoTesting(t, "GET", "/campaign/"+cid+"?deals=true", nil, &load)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
			}

			if load.Budget != 5000 {
				t.Fatal("Campaign budget did not increment!")
			}

			if len(load.Deals) <= len(oldLoad.Deals) {
				t.Fatal("Deals did not increase!")
			}

			// Lets see if it's new store was affected!
			store := budget.Store{}
			r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cid, nil, &store)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
			}

			if store.Budget <= oldStore.Budget {
				t.Fatal("New store budget did not increase!")
			}

			if store.Spendable <= oldStore.Spendable {
				t.Fatal("Spendable did not increase!")
			}
		}

		if i == 7 {
			decreaseCid = cid
			// For the last campaign lets decrease the budget!
			trueVal := true
			budgetVal := float64(150)
			name := "The day wlker"
			upd := &CampaignUpdate{Status: &trueVal, Male: &trueVal, Female: &trueVal, Budget: &budgetVal, Name: &name}
			r = rst.DoTesting(t, "PUT", "/campaign/"+cid, upd, nil)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
				return
			}

			var load common.Campaign
			r = rst.DoTesting(t, "GET", "/campaign/"+cid, nil, &load)
			if r.Status != 200 {
				t.Fatal("Bad status code!")
			}

			if load.Budget != 150 {
				t.Fatal("Campaign budget did not decrease!")
			}
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

	// LETS RUN BILLING!
	r = rst.DoTesting(t, "GET", "/billing?force=1&dbg=1", nil, nil)
	if r.Status != 200 {
		t.Fatal("Bad status code!", string(r.Value))
	}

	// Lets see what happened to stores!
	for cid, store := range cids {
		var newStore budget.Store
		r = rst.DoTesting(t, "GET", "/getBudgetInfo/"+cid, nil, &newStore)
		if r.Status != 200 {
			t.Fatal("Bad status code!")
		}

		if newStore.Spent > 0 {
			t.Fatal("Bad new store values!")
		}

		if newStore.ExchangeFee != 0.2 {
			t.Fatal("Bad exchange fee!")
		}

		if int32(store.Spendable) != int32(newStore.Leftover) {
			t.Fatal("Didn't carry over leftover from last month!")
		}

		if int32(newStore.Spendable) != int32(newStore.Leftover+newStore.Budget) {
			t.Fatal("Incorrect spendable calculation!")
		}

		if cid == decreaseCid {
			// This is the campaign that was decreased!
			if newStore.Budget != 150 {
				t.Fatal("Store does not have updated budget!")
			}
		}
	}
}
