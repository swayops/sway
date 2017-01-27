package swipe // Stripe combined with sway.. get it?

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/currency"
	"github.com/stripe/stripe-go/plan"
	"github.com/stripe/stripe-go/sub"

	"github.com/swayops/sway/internal/subscriptions"
	"github.com/swayops/sway/misc"
)

type Subscription struct {
	SubscriptionID string  `json:"subID,omitempty"`     // Stripe Subscription ID
	Plan           int     `json:"plan,omitempty"`      // Sway Plan Type (only needed for Sway agency advertisers)
	Price          float64 `json:"price,omitempty"`     // NOTE: This is only used for enterprise since the price is negotiated there
	Monthly        bool    `json:"isMonthly,omitempty"` // If false, we assume it's the yearly plan

}

func AddSubscription(name, id, custID string, newSub *Subscription) (string, error) {
	var planKey string
	if newSub.Plan == subscriptions.ENTERPRISE {
		// This is an enterprise plan.. which means it has it's own unique plan!
		planKey = "Enterprise - " + id + " - " + name + " - " + misc.PseudoUUID()
		planParams := &stripe.PlanParams{
			ID:          planKey,
			Name:        planKey,
			Amount:      uint64(newSub.Price * 100),
			Currency:    currency.USD,
			Interval:    plan.Month,
			TrialPeriod: 14,
		}
		if newSub.Monthly {
			planParams.Interval = plan.Month
		} else {
			planParams.Interval = plan.Year
		}

		_, err := plan.New(planParams)
		if err != nil {
			return "", err
		}
	} else {
		swayPlan := subscriptions.GetPlan(newSub.Plan)
		if swayPlan == nil {
			return "", ErrUnknownPlan
		}

		planKey = swayPlan.GetKey(newSub.Monthly)
	}

	if planKey == "" {
		return "", ErrUnknownPlan
	}

	subParams := &stripe.SubParams{
		Customer: custID,
		Plan:     planKey,
	}

	target, err := sub.New(subParams)
	if err != nil {
		return "", err
	}

	return target.ID, nil
}

func UpdateSubscription(name, id, custID, oldSub string, newSub *Subscription) (string, error) {
	var planKey string
	if newSub.Plan == subscriptions.ENTERPRISE {
		if newSub.Price == 0 {
			return "", ErrPrice
		}

		// This is an enterprise plan.. which means it has it's own unique plan!
		planKey = "Enterprise - " + id + " - " + name + " - " + misc.PseudoUUID()
		planParams := &stripe.PlanParams{
			ID:          planKey,
			Name:        planKey,
			Amount:      uint64(newSub.Price * 100),
			Currency:    currency.USD,
			Interval:    plan.Month,
			TrialPeriod: 14,
		}
		plan.New(planParams)
	} else {
		swayPlan := subscriptions.GetPlan(newSub.Plan)
		if swayPlan == nil {
			return "", ErrUnknownPlan
		}

		planKey = swayPlan.GetKey(newSub.Monthly)
	}

	if planKey == "" {
		return "", ErrUnknownPlan
	}

	subParams := &stripe.SubParams{
		Customer: custID,
		Plan:     planKey,
	}

	target, err := sub.Update(oldSub, subParams)
	if err != nil {
		return "", err
	}

	return target.ID, nil
}

func GetSubscription(subID string) (price float64, monthly bool, err error) {
	target, err := sub.Get(subID, nil)
	if err != nil {
		return
	}
	if target.Plan != nil {
		price = float64(target.Plan.Amount / 100)
		monthly = target.Plan.Interval == plan.Month
	}
	return
}

func CancelSubscription(oldSub string) error {
	_, err := sub.Cancel(oldSub, nil)
	return err
}
