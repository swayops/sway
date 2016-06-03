package main

import (
	"testing"

	"github.com/swayops/resty"
	"github.com/swayops/sway/internal/auth"
)

func TestAdminLogin(t *testing.T) {
	rst := getClient()
	defer putClient(rst)
	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signIn", `{"email": "admin@swayops.com", "pass": "Rf_jv9hM3-"}`, 200, `{"id":"1","status":"success"}`},
		{"GET", "/apiKey", nil, 200, nil},
		{"GET", "/signOut", nil, 200, nil},
		{"GET", "/apiKey", nil, 401, nil},
	} {
		tr.Run(t, rst)
	}
}

func TestAdvertiser(t *testing.T) {
	rst := getClient()
	defer putClient(rst)
	adv := getSignupUser()
	adv.Advertiser = &auth.Advertiser{
		DspFee:      0.5,
		ExchangeFee: 0.2,
	}
	for _, tr := range [...]*resty.TestRequest{
		{"POST", "/signUp", adv, 200, `{"id":"4","status":"success"}`},
		{"POST", "/signIn", M{"email": adv.Email, "pass": defaultPass}, 200, nil},
		{"GET", "/apiKey", nil, 200, nil},
	} {
		tr.Run(t, rst)
	}
}
