package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

type AdAgency struct {
	Id     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func (a *Auth) CreateAdAgencyTx(tx *bolt.Tx, u *User, ta *AdAgency) (err error) {
	if ta == nil || ta.Id != "" {
		return ErrUnexpected
	}

	if ta.Name == "" {
		return ErrInvalidName
	}

	if ta.Id, err = misc.GetNextIndex(tx, a.cfg.Bucket.AdAgency); err != nil {
		return
	}

	if err = u.AddItem(AdAgencyItem, ta.Id).Store(a, tx); err != nil {
		return
	}

	return misc.PutTxJson(tx, a.cfg.Bucket.AdAgency, ta.Id, ta)
}

func (a *Auth) GetAdAgencyTx(tx *bolt.Tx, taId string) *AdAgency {
	var ta AdAgency
	// ta.Id == "" is a sanity check, should never happen
	if misc.GetTxJson(tx, a.cfg.Bucket.AdAgency, taId, &ta) != nil || ta.Id == "" {
		return nil
	}
	return &ta
}

func (a *Auth) UpdateAdAgencyTx(tx *bolt.Tx, u *User, ta *AdAgency) error {
	if ta == nil || ta.Id != "" {
		return ErrUnexpected
	}
	if !u.OwnsItem(AdAgencyItem, ta.Id) {
		return ErrInvalidId
	}
	return misc.PutTxJson(tx, a.cfg.Bucket.AdAgency, ta.Id, ta)
}

func (a *Auth) DeleteAdAgencyTx(tx *bolt.Tx, u *User, taId string) error {
	if !u.OwnsItem(AdAgencyItem, taId) {
		return ErrInvalidId
	}
	if err := u.RemoveItem(AdAgencyItem, taId).Store(a, tx); err != nil {
		return err
	}
	return misc.DelBucketBytes(tx, a.cfg.Bucket.AdAgency, taId)
}
