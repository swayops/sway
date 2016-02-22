package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/swayops/sway/misc"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptRounds = 12
)

func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", nil
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptRounds)
	return string(h), err
}

func CheckPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func CreateMAC(password, token, apiKey string) string {
	tokenBytes, _ := hex.DecodeString(token)
	apiKeyBytes, _ := hex.DecodeString(apiKey)
	key := make([]byte, 0, (len(token)+len(apiKey))/2)
	key = append(key, tokenBytes...)
	key = append(key, apiKeyBytes...)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyMac(mac1, password, token, apiKey string) bool {
	mac2 := misc.DecodeHex(CreateMAC(password, token, apiKey))
	return hmac.Equal(misc.DecodeHex(mac1), mac2)
}
