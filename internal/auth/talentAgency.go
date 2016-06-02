package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
)

const (
	inviteFormat = "id::%s"
)

type TalentAgency struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`

	Fee        float64 `json:"fee,omitempty"` // Percentage (decimal)
	InviteCode string  `json:"inviteCode,omitempty"`
}

func GetTalentAgency(u *User) *TalentAgency {
	if u == nil || u.Type != TalentAgencyScope {
		return nil
	}
	var ag TalentAgency
	if json.Unmarshal(u.Data, &ag) != nil {
		return nil
	}
	return &ag
}

func (a *Auth) GetTalentAgencyTx(tx *bolt.Tx, userID string) *TalentAgency {
	return GetTalentAgency(a.GetUserTx(tx, userID))
}

func (a *Auth) GetTalentAgency(userID string) (ag *TalentAgency) {
	a.db.View(func(tx *bolt.Tx) error {
		ag = GetTalentAgency(a.GetUserTx(tx, userID))
		return nil
	})
	return
}

func (ag *TalentAgency) setToUser(_ *Auth, u *User) error {
	if ag == nil {
		return ErrUnexpected
	}
	if u.ID == "" {
		panic("wtfmate?")
	}
	if ag.ID == "" { // initial creation
		ag.ID, ag.Name, ag.Status = u.ID, u.Name, u.Status
		ag.InviteCode = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, u.ID)))
	} else if ag.ID != u.ID {
		return ErrInvalidID
	} else {
		u.Name, u.Status = ag.Name, ag.Status
	}
	b, err := json.Marshal(ag)
	u.Data = b
	return err
}

func (ag *TalentAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	if ag.Name == "" {
		return ErrInvalidName
	}

	if ag.Fee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}
