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
	// Newly created/updated user is passed in
	if ag == nil {
		return ErrUnexpected
	}
	if u.ID == "" {
		panic("wtfmate?")
	}
	if ag.ID == "" || ag.Name == "" {
		// Initial creation:
		// Copy the newly created user's name and status to
		// the agency
		ag.Name, ag.Status = u.Name, u.Status
	} else if ag.ID != u.ID {
		return ErrInvalidID
	} else {
		// Update the user properties when the
		// agency has been updated
		u.Name, u.Status = ag.Name, ag.Status
	}
	// Make sure IDs are congruent each create/update
	ag.ID, ag.InviteCode = u.ID, common.GetCodeFromID(u.ID)
	u.TalentAgency = ag
	return nil
}

func (ag *TalentAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	if ag.Fee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}
