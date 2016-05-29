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
	adv.ID, adv.AgencyID, adv.Name, adv.Status = u.ID, u.ParentID, u.Name, u.Status
	return &adv
}

func (a *Auth) GetAdvertiserTx(tx *bolt.Tx, curUser *User, userID string) *Advertiser {
	if curUser != nil && curUser.ID == userID {
		return GetAdvertiser(curUser)
	}
	return GetAdvertiser(a.GetUserTx(tx, userID))
}

func (adv *Advertiser) setToUser(_ *Auth, u *User) error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.Name != "" {
		u.Name, u.Status = adv.Name, adv.Status
	}
	adv.ID, adv.AgencyID, adv.Name, adv.Status = "", "", "", false
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
