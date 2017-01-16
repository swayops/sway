package swipe // Stripe combined with sway.. get it?

import (
	"errors"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/currency"
	"github.com/stripe/stripe-go/plan"
	"github.com/stripe/stripe-go/subscription"

	"github.com/swayops/sway/internal/subscriptions"
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

type Subscription struct {
	SubscriptionID string  `json:"subID,omitempty"`     // Stripe Subscription ID
	Plan           int     `json:"plan,omitempty"`      // Sway Plan Type (only needed for Sway agency advertisers)
	Price          float64 `json:"price,omitempty"`     // NOTE: This is only used for enterprise since the price is negotiated there
	Monthly        bool    `json:"isMonthly,omitempty"` // If false, we assume it's the yearly plan

}

func AddSubscription(name, id, custID string, sub *Subscription) (string, error) {
	var planKey string
	if sub.Plan == subscriptions.ENTERPRISE {
		if sub.Price == 0 {
			return "", ErrPrice
		}

		// This is an enterprise plan.. which means it has it's own unique plan!
		planKey = "Enterprise - " + id + " - " + name
		planParams := &stripe.PlanParams{
			ID:          key,
			Name:        key,
			Amount:      sub.Price,
			Currency:    currency.USD,
			Interval:    plan.Month,
			TrialPeriod: 14,
		}
		if sub.Monthly {
			planParams.Interval = plan.Month
		} else {
			planParams.Interval = plan.Year
		}

		plan.New(planParams)
	} else {
		swayPlan := subscriptions.GetPlan(sub.Plan)
		if swayPlan == nil {
			return "", ErrUnknownPlan
		}

		planKey = swayPlan.GetKey(sub.Monthly)
	}

	if planKey == "" {
		return "", ErrUnknownPlan
	}

	subParams := &stripe.SubParams{
		Customer: custID,
		Plan:     planKey,
	}

	target, err := subscription.New(subParams)
	if err != nil {
		return "", err
	}

	return target.ID, nil
}

func UpdateSubscription(name, id, custID, oldSub string, sub *Subscription) (string, error) {
	var planKey string
	if sub.Plan == subscriptions.ENTERPRISE {
		// This is an enterprise plan.. which means it has it's own unique plan!
		planKey = "Enterprise - " + id + " - " + name
		planParams := &stripe.PlanParams{
			ID:          key,
			Name:        key,
			Amount:      sub.MonthlyPrice,
			Currency:    currency.USD,
			Interval:    plan.Month,
			TrialPeriod: 14,
		}
		plan.New(planParams)
	} else {
		swayPlan := subscriptions.GetPlan(sub.Plan)
		if swayPlan == nil {
			return "", ErrUnknownPlan
		}

		planKey = swayPlan.GetKey()
	}

	if planKey == "" {
		return "", ErrUnknownPlan
	}

	subParams := &stripe.SubParams{
		Customer: custID,
		Plan:     planKey,
	}

	target, err := subscription.Update(oldSub, updatedSub)
	if err != nil {
		return "", err
	}

	return target.ID, nil
}

func CancelSubscription(id, custID, oldSub string) error {
	_, err := subscription.Cancel(oldSub, nil)
	return err
}
