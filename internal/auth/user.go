package auth

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

type ItemType string

// update this as we add new item types
const (
	AdAgencyItem     ItemType = "advAgency"
	AdvertiserItem   ItemType = "adv"
	CampaignItem     ItemType = "camp"
	TalentAgencyItem ItemType = "talentAgency"
	InfluencerItem   ItemType = `influencer`
)

const (
	AdminUserID           = "1"
	SwayOpsAdAgencyID     = "2"
	SwayOpsTalentAgencyID = "3"
)

type Login struct {
	UserID   string `json:"userID"`
	Password string `json:"password"`
}

type User struct {
	ID        string          `json:"id"`
	ParentID  string          `json:"parentId,omitempty"`
	Name      string          `json:"name,omitempty"`
	Email     string          `json:"email,omitempty"`
	Type      Scope           `json:"type,omitempty"`
	Phone     string          `json:"phone,omitempty"`
	Address   string          `json:"address,omitempty"`
	Status    bool            `json:"status,omitempty"`
	CreatedAt int64           `json:"createdAt,omitempty"`
	UpdatedAt int64           `json:"updatedAt,omitempty"`
	Children  []string        `json:"children,omitempty"`
	APIKey    string          `json:"apiKeys,omitempty"`
	Salt      string          `json:"salt,omitempty"`
	Data      json.RawMessage `json:"Data,omitempty"`
}

type signupUser struct {
	User
	Password  string `json:"pass"`
	Password2 string `json:"pass2"`
}

// Trim returns a browser-safe version of the User, mainly hiding salt, and maybe possibly apiKeys
func (u *User) Trim() *User {
	u.Salt = ""
	return u
}

// Update fills the updatable fields in the struct, fields like Created and ID should never be blindly set.
func (u *User) Update(o *User) *User {
	u.Name, u.Email, u.Phone, u.Address = o.Name, o.Email, o.Phone, o.Address
	u.Status = o.Status
	u.UpdatedAt = time.Now().UnixNano()
	u.Data = o.Data
	return u
}

// TODO sort and use binary search
func (u *User) AddChild(id string) *User {
	if !u.Owns(id) {
		u.Children = append(u.Children, id)
	}
	return u
}
func (u *User) Owns(id string) bool {
	return common.StringsIndexOf(u.Children, id) > -1
}

func (u *User) RemoveChild(id string) *User {
	u.Children = common.StringsRemove(u.Children, id)
	return u
}

func (u *User) Check(newUser bool) error {
	if newUser && len(u.ID) != 0 {
		return ErrInvalidUserID
	}
	if u.Name == "" {
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
	return misc.PutTxJson(tx, a.cfg.Bucket.User, u.ID, u)
}

func (a *Auth) CreateUserTx(tx *bolt.Tx, u *User, password string) (err error) {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = misc.TrimEmail(u.Email)

	if err = u.Check(true); err != nil {
		return
	}

	if err != nil {
		return
	}
	u.CreatedAt = time.Now().UnixNano()
	u.UpdatedAt = u.CreatedAt
	u.Salt = hex.EncodeToString(misc.CreateToken(SaltLen))

	if password, err = HashPassword(password); err != nil {
		return
	}

	if u.ID, err = misc.GetNextIndex(tx, a.cfg.Bucket.User); err != nil {
		return
	}

	if err = misc.PutTxJson(tx, a.cfg.Bucket.User, u.ID, u); err != nil {
		return
	}

	// logins are always in lowercase
	login := &Login{
		UserID:   u.ID,
		Password: password,
	}

	if err = misc.PutTxJson(tx, a.cfg.Bucket.Login, misc.TrimEmail(u.Email), login); err != nil {
		return
	}
	return
}

func (a *Auth) DeleteUserTx(tx *bolt.Tx, userID string) (err error) {
	user := a.GetUserTx(tx, userID)
	if user == nil {
		return ErrInvalidUserID
	}
	uid := []byte(userID)
	misc.GetBucket(tx, a.cfg.Bucket.User).Delete(uid)
	misc.GetBucket(tx, a.cfg.Bucket.Login).Delete([]byte(user.Email))
	switch user.Type {
	case AdAgencyScope:
		a.moveChildrenTx(tx, user, SwayOpsAdAgencyID)
	case TalentAgencyScope:
		a.moveChildrenTx(tx, user, SwayOpsTalentAgencyID)
	}
	return
}

func (a *Auth) GetUserTx(tx *bolt.Tx, userID string) *User {
	var u User
	if misc.GetTxJson(tx, a.cfg.Bucket.User, userID, &u) == nil && len(u.Salt) > 0 {
		return &u
	}
	return nil
}
