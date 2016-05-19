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
	ErrInvalidAgencyId  = errors.New("invalid agency id")
	ErrInvalidId        = errors.New("invalid item id")
	ErrInvalidName      = errors.New("invalid or missing name")
	ErrInvalidEmail     = errors.New("invalid or missing email")
	ErrUserExists       = errors.New("the email address already exists")
	ErrInvalidUserType  = errors.New("invalid or missing user type")
	ErrInvalidPass      = errors.New("invalid or missing password")
	ErrInvalidFee       = errors.New("invalid or missing fee")
	ErrEmailExists      = errors.New("email is already registered")
	ErrShortPass        = errors.New("password can't be less than 8 characters")
	ErrPasswordMismatch = errors.New("password mismatch")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrUnexpected       = errors.New("unexpected system error, our highly trained bug squashers have been summoned")
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
		HttpOnly: true,
		//Secure:   true,
	}
	if dur > 0 {
		cookie.Expires = time.Now().Add(dur)
	} else {
		cookie.MaxAge = -1
	}
	http.SetCookie(w, cookie)
}

func refreshCookie(w http.ResponseWriter, r *http.Request, name string, dur time.Duration) {
	c, err := r.Cookie(name)
	if err != nil {
		return
	}
	c.Path, c.Expires = "/", time.Now().Add(dur)
	http.SetCookie(w, c)
}

func getCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}

func deleteCookie(w http.ResponseWriter, name string) {
	setCookie(w, name, "deleted", -1)
}

func getOwnersKey(itemType ItemType, itemId string) []byte {
	return []byte(string(itemType) + ":" + itemId)
}

func getCreds(req *http.Request) (token, key string, isApiKey bool) {
	if token, key = getCookie(req, "token"), getCookie(req, "key"); len(token) > 0 && len(key) > 0 {
		return
	}
	apiKey := req.Header.Get(ApiKeyHeader)
	if apiKey == "" {
		apiKey = req.URL.Query().Get("key")
	}
	if len(apiKey) < 32 {
		return "", "", false
	}
	return apiKey[:32], apiKey[32:], true
}
