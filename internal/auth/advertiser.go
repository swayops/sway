package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

type Advertiser struct {
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`

	Name   string `json:"name"`
	Status string `json:"status,omitempty"`

	ExchangeFee float32 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float32 `json:"dspFee,omitempty"`      // Percentage (decimal)
}

func (a *Auth) CreateAdvertiserTx(tx *bolt.Tx, user *User, adv *Advertiser) (err error) {
	if adv == nil || adv.Id != "" {
		return ErrUnexpected
	}

	if adv.ExchangeFee == 0 || adv.ExchangeFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 || adv.DspFee > 0.99 {
		return ErrInvalidFee
	}

	if !user.OwnsItem(AdAgencyItem, adv.AgencyId) {
		return ErrInvalidAgencyId
	}

	if adv.Name == "" {
		return ErrInvalidName
	}

	if adv.Id, err = misc.GetNextIndex(tx, a.cfg.Bucket.Advertiser); err != nil {
		return
	}

	return misc.PutTxJson(tx, a.cfg.Bucket.Advertiser, adv.Id, adv)
}

func (a *Auth) GetAdvertiserTx(tx *bolt.Tx, advId string) *Advertiser {
	var adv Advertiser
	// adv.Fee == 0 is a sanity check, should never happen
	if misc.GetTxJson(tx, a.cfg.Bucket.Advertiser, advId, &adv) != nil {
		return nil
	}
	return &adv
}

func (a *Auth) UpdateAdvertiserTx(tx *bolt.Tx, u *User, adv *Advertiser) error {
	if adv == nil || adv.Id != "" || adv.AgencyId == "" {
		return ErrUnexpected
	}
	if !u.OwnsItem(AdAgencyItem, adv.AgencyId) {
		return ErrInvalidAgencyId
	}

	oAdv := a.GetAdvertiserTx(tx, adv.Id)
	if oAdv == nil || oAdv.AgencyId != adv.AgencyId {
		return ErrUnexpected
	}

	if adv.ExchangeFee == 0 || adv.ExchangeFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 || adv.DspFee > 0.99 {
		return ErrInvalidFee
	}
	return misc.PutTxJson(tx, a.cfg.Bucket.Advertiser, adv.Id, adv)
}

func (a *Auth) DeleteAdvertiserTx(tx *bolt.Tx, u *User, advId string) error {
	return misc.DelBucketBytes(tx, a.cfg.Bucket.Advertiser, advId)
}
