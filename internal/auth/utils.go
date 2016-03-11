package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInvalidUserId    = errors.New("invalid user id, hax0r")
	ErrInvalidName      = errors.New("invalid or missing name")
	ErrInvalidEmail     = errors.New("invalid or missing email")
	ErrInvalidUserType  = errors.New("invalid or missing user type")
	ErrInvalidPass      = errors.New("invalid or missing password")
	ErrEmailExists      = errors.New("email is already registered")
	ErrShortPass        = errors.New("password can't be less than 8 characters")
	ErrPasswordMismatch = errors.New("password mismatch")
	ErrUnauthorized     = errors.New("unauthorized")
)

func GetCtxUser(c *gin.Context) *User {
	if u, ok := c.Get(gin.AuthUserKey); ok {
		if u, ok := u.(*User); ok {
			return u
		}
	}
	return nil
}

func setCookie(w http.ResponseWriter, name, value string, dur time.Duration) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     name,
		Value:    value,
		Expires:  time.Now().Add(dur),
		HttpOnly: true,
		Secure:   true,
	}
	http.SetCookie(w, cookie)
}

func refreshCookie(w http.ResponseWriter, r *http.Request, name string, dur time.Duration) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return
	}
	cookie.Expires = time.Now().Add(dur)
	http.SetCookie(w, cookie)
}

func getCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}

func getOwnersKey(itemType ItemType, itemId string) []byte {
	key := make([]byte, 0, len(itemType)+1+len(itemId))
	key = append(key, itemType...)
	key = append(key, ':')
	key = append(key, itemId...)
	return key
}

func getCreds(req *http.Request) (token, key string, isApiKey bool) {
	if token, key = getCookie(req, "token"), getCookie(req, "key"); len(token) > 0 && len(key) > 0 {
		return
	}
	apiKey := req.Header.Get(ApiKeyHeader)
	if len(apiKey) != TokenStringLen+MacStringLen {
		return "", "", false
	}
	return apiKey[:TokenStringLen], apiKey[TokenStringLen:], true
}
