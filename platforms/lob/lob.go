package lob

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/swayops/sway/config"
)

const (
	lobEndpoint = "https://api.lob.com/v1/checks"
	lobTestAuth = "test_08d860361c920163e4fba655f2de5131cfb"
	lobProdAuth = "test_08d860361c920163e4fba655f2de5131cfb"
	bankAcct    = "bank_15a6b397e90cd9b"
	fromAddr    = "adr_7261cc34ecda09af"

	lobAddrEndpoint = "https://api.lob.com/v1/verify"
)

var (
	ErrAddr    = errors.New("Missing address!")
	ErrBadAddr = errors.New("Mailing address inputted is not valid. Please email engage@swayops.com with your login email / username and the address your trying to use")
)

type AddressLoad struct {
	AddressOne string `json:"address_line1"`
	AddressTwo string `json:"address_line2"`
	City       string `json:"address_city"`
	State      string `json:"address_state"`
	Country    string `json:"address_country"`
	Zip        string `json:"address_zip"`
}

type Check struct {
	Id               string  `json:"id"`
	Tracking         *Track  `json:"tracking"`
	ExpectedDelivery string  `json:"expected_delivery_date"`
	ErrorData        *Error  `json:"error"`
	Payout           float64 `json:"payout"`
}

type Track struct {
	Id string `json:"id"`
}

func CreateCheck(id, name string, addr *AddressLoad, payout float64, cfg *config.Config) (*Check, error) {
	if addr == nil || addr.AddressOne == "" {
		return nil, ErrAddr
	}

	form := url.Values{}
	form.Add("description", fmt.Sprintf("Influencer (%s) Payout", id))

	form.Add("to[name]", name)
	form.Add("to[address_line1]", addr.AddressOne)
	form.Add("to[address_line2]", addr.AddressTwo)
	form.Add("to[address_city]", addr.City)
	form.Add("to[address_state]", addr.State)
	form.Add("to[address_zip]", addr.Zip)
	form.Add("to[address_country]", addr.Country)

	form.Add("from", fromAddr)

	form.Add("bank_account", bankAcct)
	form.Add("amount", strconv.FormatFloat(payout, 'f', 6, 64))

	if !cfg.Sandbox {
		form.Add("logo", cfg.DashURL+"/"+filepath.Join(cfg.ImageUrlPath, "sway_logo.png"))
	}

	form.Add("check_bottom", "<h1 style='padding-top:4in;'>Sway Influencer Check</h1>")

	req, err := http.NewRequest("POST", lobEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	if cfg.Sandbox {
		req.SetBasicAuth(lobTestAuth, "")
	} else {
		req.SetBasicAuth(lobProdAuth, "")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)

	var check Check
	err = json.NewDecoder(resp.Body).Decode(&check)
	resp.Body.Close()
	if check.ErrorData != nil {
		return nil, errors.New(check.ErrorData.Message)
	}

	check.Payout = payout

	return &check, err
}

type Verification struct {
	Address   *AddressLoad `json:"address"`
	Message   string       `json:"message"`
	ErrorData *Error       `json:"error"`
}

type Error struct {
	Message string `json:"message"`
}

func VerifyAddress(addr *AddressLoad, sandbox bool) (*AddressLoad, error) {
	if addr == nil || addr.AddressOne == "" {
		return nil, ErrAddr
	}

	form := url.Values{}
	form.Add("address_line1", addr.AddressOne)
	form.Add("address_line2", addr.AddressTwo)
	form.Add("address_city", addr.City)
	form.Add("address_state", addr.State)
	form.Add("address_zip", addr.Zip)
	form.Add("address_country", addr.Country)

	req, err := http.NewRequest("POST", lobAddrEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	if sandbox {
		req.SetBasicAuth(lobTestAuth, "")
	} else {
		req.SetBasicAuth(lobProdAuth, "")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var verify Verification
	err = json.NewDecoder(resp.Body).Decode(&verify)

	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if len(verify.Message) > 0 {
		log.Printf("%+v: %v", addr, verify.Message)
		err = ErrBadAddr
		return nil, err
	}
	if verify.ErrorData != nil {
		log.Printf("%+v: %v", addr, verify.ErrorData.Message)
		err = ErrBadAddr
		return nil, err
	}

	return verify.Address, nil

}
