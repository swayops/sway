package swipe // Stripe combined with sway.. get it?

import (
	"strconv"
	"strings"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/currency"
	"github.com/stripe/stripe-go/customer"
)

func Charge(id, name, cid string, amount, fromBalance float64) error {
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
				"name":        name,
				"cid":         cid,
				"fromBalance": strconv.FormatFloat(fromBalance, 'f', 2, 64),
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

func Delete(id string) error {
	cust, err := customer.Get(id, nil)
	if err != nil {
		return ErrCustomer
	}

	if cust.Sources != nil && len(cust.Sources.Values) > 0 && cust.Sources.Values[0].Card != nil {
		_, err := card.Del(cust.Sources.Values[0].Card.ID, &stripe.CardParams{Customer: cust.ID})
		if err != nil {
			return err
		}
	} else {
		return ErrCreditCard
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
	FromBalance   string `json:"fromBalance"` // Amount used from balance
}

func GetBillingHistory(id, email string, sandbox bool) []*History {
	var history []*History

	if sandbox {
		// For sandbox users just get the given ID
		return getChargesFromID(id)
	}

	custList := customer.List(nil)
	for custList.Next() {
		if cust := custList.Customer(); cust != nil {
			if strings.EqualFold(cust.Email, email) {
				// Get customers by email
				history = append(history, getChargesFromID(cust.ID)...)
			}
		}
	}

	return history
}

func getChargesFromID(id string) []*History {
	var history []*History

	params := &stripe.ChargeListParams{}
	params.Filters.AddFilter("customer", "", id)
	i := charge.List(params)
	for i.Next() {
		if ch := i.Charge(); ch != nil && ch.Status == "succeeded" {
			name, _ := ch.Meta["name"]
			cid, _ := ch.Meta["cid"]
			fromBalance, _ := ch.Meta["fromBalance"]
			hist := &History{Name: name, ID: cid, Amount: ch.Amount, Created: ch.Created, TransactionID: ch.ID, CustID: id, FromBalance: fromBalance}
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

	Delete bool `json:"del,omitempty"`
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

	tmpUS := strings.ToLower(c.Country)
	if tmpUS == "united states" || tmpUS == "united states of america" || tmpUS == "usa" {
		c.Country = "US"
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
	cust, err := customer.Get(id, nil)
	if err != nil {
		return nil, ErrCustomer
	}

	if cust.Sources != nil && len(cust.Sources.Values) > 0 && cust.Sources.Values[0].Card != nil {
		card := cust.Sources.Values[0].Card
		parts := strings.Split(card.Name, " ")
		var firstName, lastName string
		if len(parts) > 1 {
			firstName = parts[0]
			lastName = parts[1]
		}

		cc := &CC{
			FirstName: firstName,
			LastName:  lastName,

			Address: card.Address1,
			City:    card.City,
			State:   card.State,
			Country: card.Country,
			Zip:     card.Zip,

			CardNumber: card.LastFour,
			CVC:        "",
			ExpMonth:   strconv.Itoa(int(card.Month)),
			ExpYear:    strconv.Itoa(int(card.Year) - 2000),
		}

		return cc, nil
	}

	return nil, nil
}
