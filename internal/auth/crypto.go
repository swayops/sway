package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/swayops/sway/misc"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptRounds = 11
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

func CreateMAC(password, token, salt string) string {
	// if we change the token size to be > 16 bytes, we'll have to decode the token/salt otherwise they will get hashed
	h := hmac.New(sha256.New, []byte(token+salt))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyMac(mac1, password, token, salt string) bool {
	mac2 := misc.DecodeHex(CreateMAC(password, token, salt))
	return hmac.Equal(misc.DecodeHex(mac1), mac2)
}
