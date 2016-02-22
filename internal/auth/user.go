package auth

import (
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

type Login struct {
	UserId   string `json:"userId"`
	Password string `json:"password"`
}

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
	APIKey    string   `json:"apiKey,omitempty"` // pretty much a cooler name for crypto salt
}

// Update fills the updatable fields in the struct, fields like Created and Id should never be blindly set.
func (u *User) Update(ou *User) *User {
	u.Name, u.Email, u.Phone, u.Address, u.Agencies = ou.Name, ou.Email, ou.Phone, ou.Address, ou.Agencies
	u.UpdatedAt = time.Now().UnixNano()
	return u
}

func (u *User) OwnsAgency(aid string) bool {
	return common.StringsIndexOf(u.Agencies, aid) > -1
}

func (u *User) RemoveAgency(aid string) *User {
	u.Agencies = common.StringsRemove(u.Agencies, aid)
	return u
}

func (u *User) Check(newUser bool) error {
	if newUser && len(u.Id) != 0 {
		return ErrInvalidUserId
	}
	if len(u.Name) < 6 {
		return ErrInvalidName
	}
	if len(u.Email) < 6 /* a@a.ab */ || strings.Index(u.Email, "@") == -1 {
		return ErrInvalidEmail
	}
	// other checks?
	return nil
}

func (a *Auth) CreateUserTx(tx *bolt.Tx, u *User, password string) (err error) {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(u.Email)

	if err = u.Check(true); err != nil {
		return
	}

	u.CreatedAt = time.Now().UnixNano()
	u.UpdatedAt = u.CreatedAt
	u.APIKey = misc.PseudoUUID()

	if password, err = HashPassword(password); err != nil {
		return
	}

	if u.Id, err = misc.GetNextIndex(tx, a.cfg.Bucket.User); err != nil {
		return
	}

	if err = misc.PutTxJson(tx, a.cfg.Bucket.User, u.Id, u); err != nil {
		return
	}

	// logins are always in lowercase
	login := &Login{
		UserId:   u.Id,
		Password: password,
	}
	if err = misc.PutTxJson(tx, a.cfg.Bucket.Login, strings.ToLower(u.Email), login); err != nil {
		return
	}
	return
}
