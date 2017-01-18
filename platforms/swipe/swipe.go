package swipe // Stripe combined with sway.. get it?

import (
	"errors"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
)

var (
	ErrAmount            = errors.New("Attempting to charge zero dollar value")
	ErrCreditCard        = errors.New("Credit card missing")
	ErrPrice             = errors.New("Invalid monthly price")
	ErrCustomer          = errors.New("Unrecognized customer")
	ErrUnknownPlan       = errors.New("Unrecognized plam")
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
