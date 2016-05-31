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
	ag.Name, ag.Status = u.Name, u.Status
	if ag.InviteCode == "" {
		ag.InviteCode = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, u.ID)))
	}
	return &ag
}

func (a *Auth) GetTalentAgencyTx(tx *bolt.Tx, curUser *User, userID string) *TalentAgency {
	if curUser != nil && curUser.ID == userID {
		return GetTalentAgency(curUser)
	}
	return GetTalentAgency(a.GetUserTx(tx, userID))
}

func (ag *TalentAgency) setToUser(_ *Auth, u *User) error {
	if ag.Name != "" {
		u.Name, u.Status = ag.Name, ag.Status
	}
	ag.Name, ag.Status = "", false
	if ag.InviteCode == "" && u.ID != "" {
		ag.InviteCode = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, u.ID)))
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

	if ag.Fee == 0 || ag.Fee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}
