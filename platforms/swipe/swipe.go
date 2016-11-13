package swipe // Stripe combined with sway.. get it?

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/currency"
	"github.com/stripe/stripe-go/customer"
)

var (
	ErrAmount            = errors.New("Attempting to charge zero dollar value")
	ErrCreditCard        = errors.New("Credit card missing")
	ErrCustomer          = errors.New("Unrecognized customer")
	ErrInvalidFirstName  = errors.New("invalid first name")
	ErrInvalidLastName   = errors.New("invalid last name")
	ErrInvalidAddress    = errors.New("invalid address")
	ErrInvalidCity       = errors.New("invalid city")
	ErrInvalidState      = errors.New("invalid state, must be two-letter representation (Example: CA)")
	ErrInvalidCountry    = errors.New("invalid country, must be two-letter representation (Example: CA)")
	ErrInvalidZipcode    = errors.New("invalid zipcode")
	ErrInvalidCardNumber = errors.New("invalid card number")
	ErrInvalidCVC        = errors.New("invalid cvc")
	ErrInvalidExpMonth   = errors.New("invalid expiration month, must be two digit representation")
	ErrInvalidExpYear    = errors.New("invalid expiration year, must be two digit representation")
)

func CreateCustomer(name, email string, cc *CC) (string, error) {
	customerParams := &stripe.CustomerParams{
		Desc:  name,
		Email: email,
	}

	customerParams.SetSource(&stripe.CardParams{
		Name:     cc.FirstName + " " + cc.LastName,
		Address1: cc.Address,
		City:     cc.City,
		State:    cc.State,
		Country:  cc.Country,
		Zip:      cc.Zip,

		Number: cc.CardNumber,
		Month:  cc.ExpMonth,
		Year:   cc.ExpYear,
		CVC:    cc.CVC,
	})

	target, err := customer.New(customerParams)
	if err != nil {
		return "", err
	}

	return target.ID, nil
}

func Charge(id, name, cid string, amount float64) error {
	if amount == 0 {
		return ErrAmount
	}

	// Expects a value in dollars
	if id == "" {
		return ErrCreditCard
	}

	cust, err := customer.Get(id, nil)
	if err != nil {
		return ErrCustomer
	}

	chargeParams := &stripe.ChargeParams{
		Amount:   uint64(amount * 100),
		Currency: currency.USD,
		Customer: cust.ID,
		Params: stripe.Params{
			Meta: map[string]string{
				"name": name,
				"cid":  cid,
			},
		},
	}

	if cust.Sources != nil && len(cust.Sources.Values) > 0 {
		chargeParams.SetSource(cust.Sources.Values[0].Card.ID)
	} else {
		return ErrCreditCard
	}

	_, err = charge.New(chargeParams)
	return err
}

func Update(id string, cc *CC) error {
	if cc == nil {
		return ErrCreditCard
	}

	updated := &stripe.CustomerParams{}
	updated.SetSource(&stripe.CardParams{
		Name:     cc.FirstName + " " + cc.LastName,
		Address1: cc.Address,
		City:     cc.City,
		State:    cc.State,
		Country:  cc.Country,
		Zip:      cc.Zip,

		Number: cc.CardNumber,
		Month:  cc.ExpMonth,
		Year:   cc.ExpYear,
		CVC:    cc.CVC,
	})

	_, err := customer.Update(id, updated)
	if err != nil {
		return err
	}

	return nil
}

type History struct {
	Name          string `json:"name"`
	ID            string `json:"id"`
	Amount        uint64 `json:"amount"`
	Created       int64  `json:"created"`
	CustID        string `json:"custID"`
	TransactionID string `json:"transactionID"`
}

func GetBillingHistory(id string) []*History {
	params := &stripe.ChargeListParams{}
	params.Filters.AddFilter("customer", "", id)

	var history []*History

	i := charge.List(params)
	for i.Next() {
		if ch := i.Charge(); ch != nil {
			name, _ := ch.Meta["name"]
			cid, _ := ch.Meta["cid"]

			hist := &History{Name: name, ID: cid, Amount: ch.Amount, Created: ch.Created, TransactionID: ch.ID, CustID: id}
			history = append(history, hist)
		}
	}

	return history
}

type CC struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`

	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
	Zip     string `json:"zip"`

	CardNumber string `json:"cardNumber"`
	CVC        string `json:"cvc"`
	ExpMonth   string `json:"expMonth"`
	ExpYear    string `json:"expYear"`
}

func (c *CC) Check() error {
	if len(c.FirstName) == 0 {
		return ErrInvalidFirstName
	}

	if len(c.LastName) == 0 {
		return ErrInvalidLastName
	}

	if len(c.Address) == 0 {
		return ErrInvalidAddress
	}

	if len(c.City) == 0 {
		return ErrInvalidCity
	}

	if len(c.State) != 2 {
		return ErrInvalidState
	}

	if len(c.Zip) == 0 {
		return ErrInvalidZipcode
	}

	if len(c.CardNumber) == 0 {
		return ErrInvalidCardNumber
	}

	if len(c.Country) != 2 {
		return ErrInvalidCountry
	}

	if lngth := len(c.CVC); lngth != 3 && lngth != 4 {
		return ErrInvalidCVC
	}

	if len(c.ExpMonth) != 2 {
		return ErrInvalidExpMonth
	}

	if len(c.ExpYear) != 2 {
		return ErrInvalidExpYear
	}

	return nil
}

func GetCleanCreditCard(id string) (*CC, error) {
	cc := &CC{}

	cust, err := customer.Get(id, nil)
	if err != nil {
		return cc, ErrCustomer
	}

	if cust.Sources != nil && len(cust.Sources.Values) > 0 && cust.Sources.Values[0].Card != nil {
		card := cust.Sources.Values[0].Card
		parts := strings.Split(card.Name, " ")
		var firstName, lastName string
		if len(parts) > 1 {
			firstName = parts[0]
			lastName = parts[1]
		}

		cc = &CC{
			FirstName: firstName,
			LastName:  lastName,

			Address: card.Address1,
			City:    card.City,
			State:   card.State,
			Country: card.Country,
			Zip:     card.Zip,

			CardNumber: card.LastFour,
			CVC:        "",
			ExpMonth:   time.Month(card.Month).String(),
			ExpYear:    strconv.Itoa(int(card.Year)),
		}
	}

	return cc, nil
}
