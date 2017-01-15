package auth

import (
	"errors"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/platforms/swipe"
)

var (
	ErrCreditCardRequired = errors.New("A credit card is required to enroll into a plan")
)

type Advertiser struct {
	ID       string `json:"id,omitempty"`
	AgencyID string `json:"agencyId,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   bool   `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)

	// Advertiser level influencer blacklist keyed on InfluencerID
	Blacklist map[string]bool `json:"blacklist,omitempty"`

	Customer string `json:"customer,omitempty"` // Stripe ID

	Subscription string `json:"subID,omitempty"` // Stripe Subscription ID

	// Tmp field used to pass in a new credit card
	CCLoad *swipe.CC `json:"ccLoad,omitempty"`

	// Tmp fields used to pass in a new subscription plan
	SubLoad *swipe.Subscription `json:"subLoad,omitempty"`
}

func GetAdvertiser(u *User) *Advertiser {
	if u == nil {
		return nil
	}
	return u.Advertiser
}

func (a *Auth) GetAdvertiserTx(tx *bolt.Tx, userID string) *Advertiser {
	return GetAdvertiser(a.GetUserTx(tx, userID))
}

func (a *Auth) GetAdvertiser(userID string) (adv *Advertiser) {
	a.db.View(func(tx *bolt.Tx) error {
		adv = GetAdvertiser(a.GetUserTx(tx, userID))
		return nil
	})
	return
}

func (adv *Advertiser) setToUser(_ *Auth, u *User) error {
	// Newly created/updated user is passed in
	var err error

	if adv == nil {
		return ErrUnexpected
	}
	if u.ID == "" {
		panic("wtfmate?")
	}
	if adv.ID == "" || adv.Name == "" {
		// Initial creation:
		// Copy the newly created user's name and status to
		// the advertiser
		adv.Name, adv.Status = u.Name, u.Status
	} else if adv.ID != u.ID {
		return ErrInvalidID
	} else {
		// Update the user properties when the
		// agency has been updated
		u.Name, u.Status = adv.Name, adv.Status
	}

	// Make sure IDs are congruent each create/update
	adv.ID, adv.AgencyID = u.ID, u.ParentID
	adv.ExchangeFee = 0.2 // Global exchange fee

	if u.Advertiser != nil && len(u.Advertiser.Blacklist) > 0 {
		if adv.Blacklist == nil {
			adv.Blacklist = map[string]bool{}
		}
		for k := range u.Advertiser.Blacklist {
			adv.Blacklist[k] = true
		}
	}

	// Generate a Stripe Customer for an advertiser who passes in
	// credit card
	if adv.CCLoad != nil {
		if adv.Customer == "" {
			// First time this advertiser is getting a credit card
			adv.Customer, err = swipe.CreateCustomer(u.Name, u.Email, adv.CCLoad)
			if err != nil {
				adv.CCLoad = nil
				return err
			}
		} else {
			// Credit card is being updated
			err = swipe.Update(adv.Customer, adv.CCLoad)
			if err != nil {
				adv.CCLoad = nil
				return err
			}
		}

		adv.CCLoad = nil
	}

	// If they just opted in for a subscription plan.. lets save that shit
	if adv.SubLoad != nil {
		if adv.Customer == "" {
			// No credit card assigned! How the hell will they sign up for a plan?
			return ErrCreditCardRequired
		}

		if adv.Subscription == "" {
			// First time this advertiser is getting a subscription
			adv.Subscription, err = swipe.AddSubscription(u.Name, u.ID, adv.Customer, adv.CCLoad)
			if err != nil {
				adv.SubLoad = nil
				return err
			}
		} else {
			// Subscription is being updated!
			// STOPPING POINT! FIGURE OUT UPDATING PLAN!
			err = swipe.UpdateSubscription(u.Name, u.ID, adv.Customer, adv.SubLoad)
			if err != nil {
				adv.SubLoad = nil
				return err
			}
		}

		adv.SubLoad = nil
	}

	u.Advertiser = adv

	return nil
}

func (adv *Advertiser) Check() error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.DspFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 {
		return ErrInvalidFee
	}

	if adv.CCLoad != nil {
		if err := adv.CCLoad.Check(); err != nil {
			return err
		}
	}

	return nil
}
