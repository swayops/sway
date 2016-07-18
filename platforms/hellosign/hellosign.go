package hellosign

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const (
	reqEndpoint    = "https://api.hellosign.com/v3/signature_request/send_with_template"
	sigReqEndpoint = "https://api.hellosign.com/v3/signature_request/"
	cancelEndpoint = "https://api.hellosign.com/v3/signature_request/cancel/"

	hsAuth     = "cfa7339e122ca1d94641391c784c923e625aeb13991d921b32a6a30b9bc06382"
	w9Template = "1cb37cfcb61c9a7c43fe944a39136144996526a7" // within US
	w8Template = "test2"                                    // international
)

var (
	ErrResponse = errors.New("Empty response!")
	ErrId       = errors.New("Influencer ID in meta datadoes not match!!")
)

type Response struct {
	Signature *SigReq `json:"signature_request"`
	ErrorData *Error  `json:"error"`
}

type SigReq struct {
	Id         string `json:"signature_request_id"`
	Url        string `json:"signing_url"`
	IsComplete bool   `json:"is_complete"`
	HasError   bool   `json:"has_error"`

	MetaData *Meta `json:"metadata"`
}

type Error struct {
	Msg string `json:"error_msg"`
}

type Meta struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func SendSignatureRequest(name, email, infId string, us, sandbox bool) (string, error) {
	form := url.Values{}

	if us {
		form.Add("template_id", w9Template)
	} else {
		form.Add("template_id", w8Template)
	}

	if sandbox {
		form.Add("test_mode", "1")
	}

	form.Add("subject", "Sway Tax Form")
	form.Add("message", "Please fill out and sign the tax form to be eligible to receive payouts via check!")
	form.Add("signers[Contractor][name]", name)
	form.Add("signers[Contractor][email_address]", email)

	form.Add("metadata[name]", name)
	form.Add("metadata[id]", infId)

	req, err := http.NewRequest("POST", reqEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(hsAuth, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)

	var hsResp Response
	err = json.NewDecoder(resp.Body).Decode(&hsResp)
	resp.Body.Close()

	if hsResp.ErrorData != nil {
		return "", errors.New(hsResp.ErrorData.Msg)
	}

	if hsResp.Signature == nil || hsResp.Signature.Id == "" {
		return "", ErrResponse
	}

	return hsResp.Signature.Id, err
}

func Cancel(sigId string) (int, error) {
	req, err := http.NewRequest("POST", cancelEndpoint+sigId, nil)
	if err != nil {
		return 500, err
	}
	req.SetBasicAuth(hsAuth, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r, err := http.DefaultClient.Do(req)
	return r.StatusCode, err
}

func HasSigned(infId, sigId string) (bool, error) {
	req, err := http.NewRequest("GET", sigReqEndpoint+sigId, nil)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(hsAuth, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)

	var hsResp Response
	err = json.NewDecoder(resp.Body).Decode(&hsResp)
	resp.Body.Close()

	if hsResp.ErrorData != nil {
		return false, errors.New(hsResp.ErrorData.Msg)
	}

	if hsResp.Signature == nil || hsResp.Signature.Id == "" {
		return false, ErrResponse
	}

	if hsResp.Signature.MetaData == nil || hsResp.Signature.MetaData.Id != infId {
		return false, ErrId
	}

	return hsResp.Signature.IsComplete && !hsResp.Signature.HasError, err
}
