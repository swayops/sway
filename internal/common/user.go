package common

import (
	"errors"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

var (
	ErrInvalidUserId = errors.New("invalid user id, hax0r")
	ErrInvalidName   = errors.New("invalid or missing name")
	ErrInvalidEmail  = errors.New("invalid or missing email")
	ErrEmailExists   = errors.New("email is already registered")
)

type User struct {
	Id        string   `json:"id"`
	Name      string   `json:"name,omitempty"`
	Email     string   `json:"email,omitempty"`
	Phone     string   `json:"phone,omitempty"`
	Address   string   `json:"address,omitempty"`
	Status    bool     `json:"status,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
	Agencies  []string `json:"agencies,omitempty"`
}

// Update fills the updatable fields in the struct, fields like Created and Id should never be blindly set.
func (u *User) Update(ou *User) *User {
	u.Name, u.Email, u.Phone, u.Address, u.Agencies = ou.Name, ou.Email, ou.Phone, ou.Address, ou.Agencies
	u.UpdatedAt = time.Now().UnixNano()
	return u
}

func (u *User) OwnsAgency(aid string) bool {
	return StringsIndexOf(u.Agencies, aid) > -1
}

func (u *User) RemoveAgency(aid string) *User {
	u.Agencies = StringsRemove(u.Agencies, aid)
	return u
}

func (u *User) Check(newUser bool) error {
	if newUser && len(u.Id) != 0 {
		return ErrInvalidUserId
	}
	if len(u.Name) < 8 {
		return ErrInvalidName
	}
	if len(u.Email) < 6 /* a@a.ab */ || strings.Index(u.Email, "@") == -1 {
		return ErrInvalidEmail
	}
	// other checks?
	return nil
}

func CreateUser(tx *bolt.Tx, u *User) (err error) {
	if err = u.Check(true); err != nil {
		return
	}
	u.Id, err = misc.GetNextIndex(tx, "users")
	u.CreatedAt = time.Now().UnixNano()
	u.UpdatedAt = u.CreatedAt
	return
}
