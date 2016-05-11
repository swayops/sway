package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

type TalentAgency struct {
	Id     string  `json:"id,omitempty"`
	UserId string  `json:"userId,omitempty"`
	Name   string  `json:"name,omitempty"`
	Fee    float32 `json:"fee,omitempty"` // Percentage (decimal)
}

func (a *Auth) CreateTalentAgencyTx(tx *bolt.Tx, u *User, ta *TalentAgency) (err error) {
	if ta == nil || ta.Id != "" {
		return ErrUnexpected
	}

	if ta.UserId != u.Id {
		return ErrInvalidUserId
	}

	if ta.Name == "" {
		return ErrInvalidName
	}

	if ta.Fee == 0 || ta.Fee > 0.99 {
		return ErrInvalidFee
	}

	if ta.Id, err = misc.GetNextIndex(tx, a.cfg.Bucket.TalentAgency); err != nil {
		return
	}

	if err = u.AddItem(TalentAgencyItem, ta.Id).Store(a, tx); err != nil {
		return
	}

	return misc.PutTxJson(tx, a.cfg.Bucket.TalentAgency, ta.Id, ta)
}

func (a *Auth) GetTalentAgencyTx(tx *bolt.Tx, taId string) *TalentAgency {
	var ta TalentAgency
	// ta.Fee == 0 is a sanity check, should never happen
	if misc.GetTxJson(tx, a.cfg.Bucket.TalentAgency, taId, &ta) != nil || ta.Fee == 0 {
		return nil
	}
	return &ta
}

func (a *Auth) UpdateTalentAgencyTx(tx *bolt.Tx, u *User, ta *TalentAgency) error {
	if ta == nil || ta.Id != "" {
		return ErrUnexpected
	}
	if !u.OwnsItem(TalentAgencyItem, ta.Id) {
		return ErrInvalidId
	}
	if ta.Fee == 0 || ta.Fee > 0.99 {
		return ErrInvalidFee
	}
	return misc.PutTxJson(tx, a.cfg.Bucket.TalentAgency, ta.Id, ta)
}

func (a *Auth) DeleteTalentAgencyTx(tx *bolt.Tx, u *User, taId string) error {
	if !u.OwnsItem(TalentAgencyItem, taId) {
		return ErrInvalidId
	}
	if err := u.RemoveItem(TalentAgencyItem, taId).Store(a, tx); err != nil {
		return err
	}
	return misc.DelBucketBytes(tx, a.cfg.Bucket.TalentAgency, taId)
}
