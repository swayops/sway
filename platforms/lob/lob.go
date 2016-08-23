package lob

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	ErrAddr = errors.New("Missing address!")
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
	Id               string `json:"id"`
	Tracking         *Track `json:"tracking"`
	ExpectedDelivery string `json:"expected_delivery_date"`
	ErrorData        *Error `json:"error"`
}

type Track struct {
	Id string `json:"id"`
}

func CreateCheck(name string, addr *AddressLoad, payout float64, sandbox bool) (*Check, error) {
	if addr == nil || addr.AddressOne == "" {
		return nil, ErrAddr
	}

	form := url.Values{}
	form.Add("description", "Influencer Payout Check")

	form.Add("to[name]", name)
	form.Add("to[address_line1]", addr.AddressOne)
	form.Add("to[address_line2]", addr.AddressTwo)
	form.Add("to[address_city]", addr.City)
	form.Add("to[address_state]", addr.State)
	form.Add("to[address_zip]", addr.Zip)
	form.Add("to[address_country]", addr.Country)

	form.Add("from", fromAddr)
	// form.Add("from[name]", "Shahzil Abid")
	// form.Add("from[address_line1]", "123 Test Street")
	// form.Add("from[address_city]", "Mountain View")
	// form.Add("from[address_state]", "CA")
	// form.Add("from[address_zip]", "94041")
	// form.Add("from[address_country]", "US")

	form.Add("bank_account", bankAcct)
	form.Add("amount", strconv.FormatFloat(payout, 'f', 6, 64))

	form.Add("logo", "http://s33.postimg.org/jidpxyzwv/test.png")
	form.Add("check_bottom", "<h1 style='padding-top:4in;'>Sway Influencer Check</h1>")

	req, err := http.NewRequest("POST", lobEndpoint, strings.NewReader(form.Encode()))
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

	var check Check
	err = json.NewDecoder(resp.Body).Decode(&check)
	resp.Body.Close()
	if check.ErrorData != nil {
		return nil, errors.New(check.ErrorData.Message)
	}

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

	var verify Verification
	err = json.NewDecoder(resp.Body).Decode(&verify)

	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if len(verify.Message) > 0 {
		err = errors.New(verify.Message)
		return nil, err
	}
	if verify.ErrorData != nil {
		err = errors.New(verify.ErrorData.Message)
		return nil, err
	}

	return verify.Address, nil

}
