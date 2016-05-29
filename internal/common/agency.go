package common

import (
	"encoding/base64"
	"strings"
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
