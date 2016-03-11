package auth

import (
	"bytes"
	"encoding/hex"
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
	Type      Scope    `json:"type,omitempty"`
	Phone     string   `json:"phone,omitempty"`
	Address   string   `json:"address,omitempty"`
	Active    bool     `json:"active,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
	Agencies  []string `json:"agencies,omitempty"`
	APIKeys   []string `json:"apiKeys,omitempty"`
	Salt      string   `json:"salt,omitempty"`
}

// Trim returns a browser-safe version of the User, mainly hiding salt, and maybe possibly apiKeys
func (u *User) Trim() *User {
	u.Salt = ""
	return u
}

// Update fills the updatable fields in the struct, fields like Created and Id should never be blindly set.
func (u *User) Update(ou *User) *User {
	u.Name, u.Email, u.Phone, u.Address, u.Agencies = ou.Name, ou.Email, ou.Phone, ou.Address, ou.Agencies
	u.Active = ou.Active
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
	if !u.Type.Valid() {
		return ErrInvalidUserType
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
	u.Salt = hex.EncodeToString(misc.CreateToken(TokenLen - 8))

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

func (a *Auth) DelUserTx(tx *bolt.Tx, userId string) (err error) {
	user := a.GetUserTx(tx, userId)
	if user == nil {
		return ErrInvalidUserId
	}
	uid := []byte(userId)
	misc.GetBucket(tx, a.cfg.Bucket.User).Delete(uid)
	misc.GetBucket(tx, a.cfg.Bucket.Login).Delete([]byte(strings.ToLower(user.Email)))
	os := misc.GetBucket(tx, a.cfg.Bucket.Ownership)
	os.ForEach(func(k, v []byte) error {
		if bytes.Compare(v, uid) == 0 {
			os.Delete(k)
		}
		return nil
	})
	return
}
