package main

import (
	"testing"

	"github.com/swayops/resty"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/misc"
	"github.com/swayops/sway/server"
)

var (
	adminReq             = M{"email": server.AdminEmail, "pass": "Rf_jv9hM3-"}
	adminAdAgencyReq     = M{"email": server.AdAdminEmail, "pass": "Rf_jv9hM3-"}
	adminTalentAgencyReq = M{"email": server.TalentAdminEmail, "pass": "Rf_jv9hM3-"}
)

func TestAdminLogin(t *testing.T) {
	rst := getClient()
	defer putClient(rst)
	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signIn", adminReq, 200, misc.StatusOK("1")},
		{"GET", "/apiKey", nil, 200, nil},
		{"GET", "/signOut", nil, 200, nil},
		{"GET", "/apiKey", nil, 401, nil},
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
		{"POST", "/signIn", `{"email": "admin@swayops.com", "pass": "Rf_jv9hM3-"}`, 200, misc.StatusOK("1")},

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

func TestNewAdvertiser(t *testing.T) {
	rst := getClient()
	defer putClient(rst)
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}
	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp", adv, 200, misc.StatusOK(adv.ExpID)},
		{"POST", "/signIn", M{"email": adv.Email, "pass": defaultPass}, 200, nil},
		{"GET", "/advertiser/" + adv.ExpID, nil, 200, &auth.Advertiser{AgencyID: auth.SwayOpsAdAgencyID, DspFee: 0.5}},
	} {
		tr.Run(t, rst)
	}
}
