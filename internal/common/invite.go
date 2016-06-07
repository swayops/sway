package common

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	inviteFormat = "id::%s"
)

func GetIDFromInvite(code string) string {
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

func GetCodeFromID(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(inviteFormat, id)))
}
