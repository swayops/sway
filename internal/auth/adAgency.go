package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
)

type AdAgency struct {
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func GetAdAgency(u *User) *AdAgency {
	if u.Type != AdAgencyScope {
		return nil
	}
	var ag AdAgency
	if json.Unmarshal(u.Meta, &ag) != nil {
		return nil
	}
	return &ag
}

func (ag *AdAgency) setToUser(u *User) {
	b, _ := json.Marshal(ag)
	u.Meta = b
}

func (ag *AdAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	if ag.Name == "" {
		return ErrInvalidName
	}

	return nil
}

func (a *Auth) setAdAgencyTx(tx *bolt.Tx, u *User, ag *AdAgency) error {
	if err := ag.Check(); err != nil {
		return err
	}
	ag.setToUser(u)

	return u.Store(a, tx)
}
