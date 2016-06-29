package server

import (
	"testing"

	"github.com/swayops/resty"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
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
			Name:      "John Smith",
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
		{"GET", "/setCategory/" + inf.ExpID + "/vlogger", nil, 200, nil},
		{"GET", "/setPlatform/" + inf.ExpID + "/twitter/" + "SwayOps_com", nil, 200, nil},
		{"POST", "/setGeo/" + inf.ExpID, misc.GeoRecord{City: "hell"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   ag.ExpID,
			"categories": []string{"vlogger"},
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
			Name:      "John Smith",
			Gender:    "unicorn",
			Geo:       &misc.GeoRecord{},
			TwitterId: "justinbieber",
		},
	}

	badInf := getSignupUser()
	badInf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Name:   "John Smith",
			Gender: "purple",
			Geo:    &misc.GeoRecord{},
		},
	}

	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp", inf, 200, misc.StatusOK(inf.ExpID)},
		{"POST", "/signIn", M{"email": inf.Email, "pass": defaultPass}, 200, nil},

		// update
		{"GET", "/setCategory/" + inf.ExpID + "/vlogger", nil, 200, nil},
		{"GET", "/setPlatform/" + inf.ExpID + "/twitter/" + "SwayOps_com", nil, 200, nil},
		{"POST", "/setGeo/" + inf.ExpID, misc.GeoRecord{City: "hell"}, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"geo":        M{"city": "hell"},
			"categories": []string{"vlogger"},
			"twitter":    M{"id": "SwayOps_com"},
		}},

		// Add a social media platofrm
		{"GET", "/setPlatform/" + inf.ExpID + "/facebook/" + "justinbieber", nil, 200, nil},
		{"GET", "/influencer/" + inf.ExpID, nil, 200, M{
			"agencyId":   auth.SwayOpsTalentAgencyID,
			"geo":        M{"city": "hell"},
			"categories": []string{"vlogger"},
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
			Name:       "John Smith",
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
		Fee: 0.2,
	}

	inf := getSignupUser()
	inf.InfluencerLoad = &auth.InfluencerLoad{ // ugly I know
		InfluencerLoad: influencer.InfluencerLoad{
			Name:       "John Smith",
			Gender:     "m",
			Geo:        &misc.GeoRecord{},
			InviteCode: common.GetCodeFromID(ag.ExpID),
			TwitterId:  "justinbieber",
		},
	}

	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.2,
		ExchangeFee: 0.2,
	}

	cmp := common.Campaign{
		Status:       true,
		AdvertiserId: adv.ExpID,
		Budget:       1000.5,
		Name:         "The Day Walker",
		Twitter:      true,
		Gender:       "mf",
		Link:         "blade.org",
		Tags:         []string{"#mmmm"},
		Whitelist: &common.TargetList{
			Twitter: []string{"justinbieber"},
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
		{"GET", "/getDeals/" + inf.ExpID + "/0/0", nil, 200, `{"campaignId": "` + cmp.Id + `"}`},
	} {

		tr.Run(t, rst)
	}
}

func TestBudget(t *testing.T) {
}

func TestReporting(t *testing.T) {
}
