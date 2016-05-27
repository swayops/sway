package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
)

type TalentAgency struct {
	Name   string  `json:"name,omitempty"`
	Fee    float64 `json:"fee,omitempty"` // Percentage (decimal)
	Status bool    `json:"status,omitempty"`
}

func GetTalentAgency(u *User) *TalentAgency {
	if u.Type != TalentAgencyScope {
		return nil
	}
	var ag TalentAgency
	if json.Unmarshal(u.Meta, &ag) != nil {
		return nil
	}
	return &ag
}

func (ag *TalentAgency) setToUser(u *User) {
	b, _ := json.Marshal(ag)
	u.Meta = b
}

func (ag *TalentAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	if ag.Name == "" {
		return ErrInvalidName
	}

	if ag.Fee == 0 || ag.Fee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}

func (a *Auth) setTalentAgencyTx(tx *bolt.Tx, u *User, ag *TalentAgency) error {
	if err := ag.Check(); err != nil {
		return err
	}
	ag.setToUser(u)

	return u.Store(a, tx)
}
