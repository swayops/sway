package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
)

type Advertiser struct {
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)
}

func GetAdvertiser(u *User) *Advertiser {
	if u.Type != AdvertiserScope {
		return nil
	}
	var adv Advertiser
	if json.Unmarshal(u.Meta, &adv) != nil || adv.Name == "" {
		return nil
	}
	return &adv
}

func (adv *Advertiser) setToUser(u *User) {
	b, _ := json.Marshal(adv)
	u.Meta = b
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

func (a *Auth) setAdvertiserTx(tx *bolt.Tx, user *User, adv *Advertiser) error {
	if err := adv.Check(); err != nil {
		return err
	}

	adv.setToUser(user)

	return user.Store(a, tx)
}
