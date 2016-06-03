package main

import "testing"

func TestAdminLogin(t *testing.T) {
	rst := getClient()
	defer putClient(rst)
	for _, tr := range [...]*testReq{
		{"POST", "/signIn", `{"email": "admin@swayops.com", "pass": "Rf_jv9hM3-"}`, R(200, `{"id":"1","status":"success"}`)},
		{"GET", "/apiKey", nil, R(200, nil)},
	} {
		tr.run(t, rst)
	}
}
