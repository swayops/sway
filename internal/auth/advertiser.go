package auth

import "github.com/boltdb/bolt"

type Advertiser struct {
	ID       string `json:"id,omitempty"`
	AgencyID string `json:"agencyId,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   bool   `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)

	// Advertiser level influencer blacklist keyed on InfluencerID
	Blacklist map[string]bool `json:"blacklist,omitempty"`
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
	u.Advertiser = adv

	return nil
}

func (adv *Advertiser) Check() error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.ExchangeFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 {
		return ErrInvalidFee
	}

	return nil
}
