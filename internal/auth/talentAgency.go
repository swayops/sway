package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
)

type TalentAgency struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`

	Fee        float64 `json:"fee,omitempty"` // Percentage (decimal)
	InviteCode string  `json:"inviteCode,omitempty"`
}

func GetTalentAgency(u *User) *TalentAgency {
	if u == nil {
		return nil
	}
	return u.TalentAgency
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
	} else if ag.ID != u.ID {
		return ErrInvalidID
	} else {
		u.Name, u.Status = ag.Name, ag.Status
	}
	ag.InviteCode = common.GetCodeFromID(u.ID)
	u.TalentAgency = ag
	return nil
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
