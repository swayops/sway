package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
)

const (
	inviteFormat = "id::%s"
)

type TalentAgency struct {
	Name   string  `json:"name,omitempty"`
	Fee    float64 `json:"fee,omitempty"` // Percentage (decimal)
	Status bool    `json:"status,omitempty"`

	InviteCode string `json:"inviteCode,omitempty"`
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

func (ag *TalentAgency) SetInviteCode(u *User) {
	ag.InviteCode = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, u.ID)))
}

func GetIdFromInvite(code string) string {
	dec, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return ""
	}

	parts := strings.Split(string(dec), "::")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func (a *Auth) setTalentAgencyTx(tx *bolt.Tx, u *User, ag *TalentAgency) error {
	if err := ag.Check(); err != nil {
		return err
	}
	ag.setToUser(u)

	return u.Store(a, tx)
}
