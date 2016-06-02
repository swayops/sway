package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
)

type AdAgency struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func GetAdAgency(u *User) *AdAgency {
	if u == nil || u.Type != AdAgencyScope {
		return nil
	}
	var ag AdAgency
	if json.Unmarshal(u.Data, &ag) != nil {
		return nil
	}
	return &ag
}

func (a *Auth) GetAdAgencyTx(tx *bolt.Tx, userID string) *AdAgency {
	return GetAdAgency(a.GetUserTx(tx, userID))
}

func (a *Auth) GetAdAgency(userID string) (ag *AdAgency) {
	a.db.View(func(tx *bolt.Tx) error {
		ag = GetAdAgency(a.GetUserTx(tx, userID))
		return nil
	})
	return
}

func (ag *AdAgency) setToUser(_ *Auth, u *User) error {
	if ag == nil {
		return ErrUnexpected
	}
	if u.ID == "" {
		panic("wtfmate?")
	}
	if ag.ID == "" { // initial creation
		ag.ID, ag.Name, ag.Status = u.ID, u.Name, u.Status
	} else if ag.ID != u.ID {
		return ErrInvalidID
	} else {
		u.Name, u.Status = ag.Name, ag.Status
	}
	b, err := json.Marshal(ag)
	u.Data = b
	return err
}

func (ag *AdAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	return nil
}
