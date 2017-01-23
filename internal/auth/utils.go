package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/misc"
)

var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInvalidUserID    = errors.New("invalid user id")
	ErrInvalidParentID  = errors.New("invalid parent id")
	ErrInvalidAgencyID  = errors.New("invalid agency id")
	ErrInvalidID        = errors.New("invalid item id")
	ErrInvalidName      = errors.New("invalid or missing name (must be full name)")
	ErrSubUser          = errors.New("only the main account owner can request a change password")
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
	ErrBadGender        = errors.New("Please provide a gender ('m' or 'f')")
	ErrNoAgency         = errors.New("Please provide an agency id")
	ErrNoGeo            = errors.New("Please provide a geo")
	ErrNoName           = errors.New("Please provide a name")
	ErrBadCat           = errors.New("Please provide a valid category")
	ErrName             = errors.New("Please provide a valid name")
	ErrPlatform         = errors.New("Please provide atleast one social media platform id")
)

func GetCtxUser(c *gin.Context) *User {
	if u, ok := c.Get(gin.AuthUserKey); ok {
		if u, ok := u.(*User); ok {
			return u
		}
	}
	return nil
}

func IsSubUser(c *gin.Context) bool {
	_, isu := c.Get(IsSubUserKey)
	return isu
}

func getOwnersKey(itemType ItemType, itemID string) []byte {
	return []byte(string(itemType) + ":" + itemID)
}

func getCreds(req *http.Request) (token, key string, isApiKey bool) {
	if token, key = misc.GetCookie(req, "token"), misc.GetCookie(req, "key"); len(token) > 0 && len(key) > 0 {
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

type SpecUser interface {
	Check() error
	setToUser(*Auth, *User) error
}
