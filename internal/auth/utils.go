package auth

import (
	"errors"
	"net/http"
	"time"
)

var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInvalidUserId    = errors.New("invalid user id, hax0r")
	ErrInvalidName      = errors.New("invalid or missing name")
	ErrInvalidEmail     = errors.New("invalid or missing email")
	ErrInvalidPass      = errors.New("invalid or missing password")
	ErrEmailExists      = errors.New("email is already registered")
	ErrShortPass        = errors.New("password can't be less than 8 characters")
	ErrPasswordMismatch = errors.New("password mismatch")
	ErrUnauthorized     = errors.New("unauthorized")
)

func setCookie(w http.ResponseWriter, name, value string, dur time.Duration) {
	cookie := &http.Cookie{
		Path:    "/",
		Name:    name,
		Value:   value,
		Expires: time.Now().Add(dur),
	}
	http.SetCookie(w, cookie)
}

func refreshCookie(w http.ResponseWriter, r *http.Request, name string, dur time.Duration) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return
	}
	cookie.Expires = time.Now().Add(dur)
	cookie.Path = "/"
	http.SetCookie(w, cookie)
}

func getCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}

func getOwnersKey(itemType, itemId string) []byte {
	key := make([]byte, 0, len(itemType)+1+len(itemId))
	key = append(key, itemType...)
	key = append(key, ':')
	key = append(key, itemId...)
	return key
}
