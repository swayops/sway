package common

import (
	b64 "encoding/base64"
	"fmt"
	"strings"
)

const (
	inviteFormat               = "id::%s"
	DEFAULT_SWAY_TALENT_AGENCY = "1"
)

type TalentAgency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Fee    float32 `json:"fee,omitempty"` // Percentage (decimal)
	Status bool    `json:"status,omitempty"`

	InviteCode string `json:"inviteCode,omitempty"`
}

func (t *TalentAgency) SetInviteCode() {
	t.InviteCode = b64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, t.Id)))
}

func GetIdFromInvite(code string) string {
	dec, err := b64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return ""
	}

	parts := strings.Split(string(dec), "::")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
