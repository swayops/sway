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
	lobEndpoint         = "https://api.lob.com/v1/checks"
	lobDomesticEndpoint = "https://api.lob.com/v1/us_verifications"
	lobIntlEndpoint     = "https://api.lob.com/v1/intl_verifications"
)

var (
	ErrAddr  = errors.New("Missing address")
	ErrState = errors.New("State must be 2 letter representation")

	ErrBadAddr = errors.New("Mailing address inputted is not valid. Please email engage@swayops.com with your login email / username and the address you're trying to use")
)

type AddressLoad struct {
	AddressOne string `json:"address_line1"`
	AddressTwo string `json:"address_line2"`
	City       string `json:"address_city"`
	State      string `json:"address_state"`
	Country    string `json:"address_country"`
	Zip        string `json:"address_zip"`
}

func (a *AddressLoad) String() string {
	var out string
	out += a.AddressOne + ", "

	if a.AddressTwo != "" {
		out += a.AddressTwo + ", "
	}

	out += a.City + ", " + a.State + ", " + a.Country

	if a.Zip != "" {
		out += ", " + a.Zip
	}

	return out
}

type Check struct {
	Id               string  `json:"id"`
	Tracking         *Track  `json:"tracking"`
	ExpectedDelivery string  `json:"expected_delivery_date"`
	ErrorData        *Error  `json:"error"`
	Payout           float64 `json:"payout"`
}

type Error struct {
	Message string `json:"message"`
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

	form.Add("from", cfg.Lob.Addr)
	form.Add("bank_account", cfg.Lob.BankAcct)

	form.Add("amount", strconv.FormatFloat(payout, 'f', 6, 64))

	if !cfg.Sandbox {
		form.Add("logo", cfg.DashURL+"/"+filepath.Join(cfg.ImageUrlPath, "sway_logo.png"))
	}

	form.Add("check_bottom", "<h1 style='padding-top:4in;'>Sway Influencer Check</h1>")

	req, err := http.NewRequest("POST", lobEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cfg.Lob.Key, "")

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

type IntlVerification struct {
	Address   *AddressLoad `json:"address"`
	Message   string       `json:"message"`
	ErrorData *Error       `json:"error"`
}

type DomesticVerification struct {
	Deliverability string `json:"deliverability"`
}

func VerifyAddress(addr *AddressLoad, cfg *config.Config) (*AddressLoad, error) {
	if addr == nil || addr.AddressOne == "" {
		return nil, ErrAddr
	}

	cy := strings.ToLower(addr.Country)
	if cy == "un" || cy == "united states" || cy == "usa" {
		// Accounting for different versions of US
		addr.Country = "US"
	}

	if addr.Country == "US" {
		return verifyUS(addr, cfg)
	} else {
		return verifyIntl(addr, cfg)
	}
}

func verifyUS(addr *AddressLoad, cfg *config.Config) (*AddressLoad, error) {
	if len(addr.State) != 2 {
		return nil, ErrState
	}

	form := url.Values{}
	form.Add("primary_line", addr.AddressOne)
	form.Add("secondary_line", addr.AddressTwo)
	form.Add("city", addr.City)
	form.Add("state", addr.State)
	form.Add("zip_code", addr.Zip)

	req, err := http.NewRequest("POST", lobDomesticEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cfg.Lob.Key, "")

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var verify DomesticVerification
	err = json.NewDecoder(resp.Body).Decode(&verify)

	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if verify.Deliverability != "deliverable" {
		return nil, ErrBadAddr
	}

	return addr, nil
}

func verifyIntl(addr *AddressLoad, cfg *config.Config) (*AddressLoad, error) {
	form := url.Values{}
	form.Add("address_line1", addr.AddressOne)
	form.Add("address_line2", addr.AddressTwo)
	form.Add("address_city", addr.City)
	form.Add("address_state", addr.State)
	form.Add("address_zip", addr.Zip)
	form.Add("address_country", addr.Country)

	req, err := http.NewRequest("POST", lobIntlEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cfg.Lob.Key, "")

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var verify IntlVerification
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
