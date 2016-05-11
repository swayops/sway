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

const (
	AdminUserId     = "1"
	SwayOpsAgencyId = "2"
)

type Login struct {
	UserId   string `json:"userId"`
	Password string `json:"password"`
}

type User struct {
	Id        string   `json:"id"`
	ParentId  string   `json:"parentId,omitempty"` // who created this user
	Name      string   `json:"name,omitempty"`
	Email     string   `json:"email,omitempty"`
	Type      Scope    `json:"type,omitempty"`
	Phone     string   `json:"phone,omitempty"`
	Address   string   `json:"address,omitempty"`
	Active    bool     `json:"active,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
	Items     []string `json:"items,omitempty"`
	APIKey    string   `json:"apiKeys,omitempty"`
	Salt      string   `json:"salt,omitempty"`
}

type SignupUser struct {
	User
	Password  string `json:"pass"`
	Password2 string `json:"pass2"`
}

// Trim returns a browser-safe version of the User, mainly hiding salt, and maybe possibly apiKeys
func (u *User) Trim() *User {
	u.Salt = ""
	return u
}

// Update fills the updatable fields in the struct, fields like Created and Id should never be blindly set.
func (u *User) Update(o *User) *User {
	u.Name, u.Email, u.Phone, u.Address, u.Items = o.Name, o.Email, o.Phone, o.Address, o.Items
	u.Active = o.Active
	u.UpdatedAt = time.Now().UnixNano()
	return u
}

// TODO sort and use binary search
func (u *User) AddItem(it ItemType, id string) *User {
	if !u.OwnsItem(it, id) {
		u.Items = append(u.Items, string(it)+":"+id)
	}
	return u
}
func (u *User) OwnsItem(it ItemType, id string) bool {
	return common.StringsIndexOf(u.Items, string(it)+":"+id) > -1
}

func (u *User) RemoveItem(it ItemType, id string) *User {
	u.Items = common.StringsRemove(u.Items, string(it)+":"+id)
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

func (u *User) Store(a *Auth, tx *bolt.Tx) error {
	return misc.PutTxJson(tx, a.cfg.Bucket.User, u.Id, u)
}

func (a *Auth) CreateUserTx(tx *bolt.Tx, u *User, password string) (err error) {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(u.Email)

	if err = u.Check(true); err != nil {
		return
	}

	u.CreatedAt = time.Now().UnixNano()
	u.UpdatedAt = u.CreatedAt
	u.Salt = hex.EncodeToString(misc.CreateToken(SaltLen))

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

	if err = misc.PutTxJson(tx, a.cfg.Bucket.Login, misc.TrimEmail(u.Email), login); err != nil {
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
	misc.GetBucket(tx, a.cfg.Bucket.Login).Delete([]byte(misc.TrimEmail(user.Email)))
	os := misc.GetBucket(tx, a.cfg.Bucket.Ownership)
	os.ForEach(func(k, v []byte) error {
		if bytes.Compare(v, uid) == 0 {
			os.Delete(k)
		}
		return nil
	})
	return
}

func (a *Auth) GetUserTx(tx *bolt.Tx, userId string) *User {
	var u User
	if misc.GetTxJson(tx, a.cfg.Bucket.User, userId, &u) == nil && len(u.Salt) > 0 {
		return &u
	}
	return nil
}
