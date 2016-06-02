package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
)

type Advertiser struct {
	ID       string `json:"id,omitempty"`
	AgencyID string `json:"agencyId,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   bool   `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)
}

func GetAdvertiser(u *User) *Advertiser {
	if u == nil || u.Type != AdvertiserScope {
		return nil
	}
	var adv Advertiser
	if json.Unmarshal(u.Data, &adv) != nil || adv.ExchangeFee == 0 {
		return nil
	}
	return &adv
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
	if adv == nil || u.Type == AdvertiserScope {
		return ErrUnexpected
	}

	if adv.ID == "" { // initial creation
		adv.ID, adv.AgencyID, adv.Name, adv.Status = u.ID, u.ParentID, u.Name, u.Status
	} else if adv.ID != u.ID {
		return ErrInvalidID
	} else {
		u.Name, u.Status = adv.Name, adv.Status
	}
	b, err := json.Marshal(adv)
	u.Data = b
	return err
}

func (adv *Advertiser) Check() error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.ExchangeFee == 0 || adv.ExchangeFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 || adv.DspFee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}
