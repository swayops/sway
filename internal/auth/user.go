package auth

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/boltdb/bolt"
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
	ID        string `json:"id"`
	ParentID  string `json:"parentId,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Address   string `json:"address,omitempty"`
	Status    bool   `json:"status,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	APIKey    string `json:"apiKeys,omitempty"`
	Salt      string `json:"salt,omitempty"`
	Admin     bool   `json:"admin,omitempty"`
	//	Data      json.RawMessage `json:"Data,omitempty"`

	AdAgency     *AdAgency     `json:"adAgency,omitempty"`
	TalentAgency *TalentAgency `json:"talentAgency,omitempty"`
	Advertiser   *Advertiser   `json:"advertiser,omitempty"`
	Influencer   *Influencer   `json:"inf,omitempty"`

	//special hack, the gods will look down upon us and spit
	InfluencerLoad *InfluencerLoad `json:"influencer,omitempty"`
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
	return u
}

func (u *User) UpdateData(a *Auth, su SpecUser) error {
	utype := u.Type()
	switch su.(type) {
	case *AdAgency:
		if utype != AdAgencyScope {
			return ErrInvalidUserType
		}
	case *TalentAgency:
		if utype != TalentAgencyScope {
			return ErrInvalidUserType
		}
	case *Advertiser:
		if utype != AdvertiserScope {
			return ErrInvalidUserType
		}
	case *InfluencerLoad:
		if utype != InfluencerScope {
			return ErrInvalidUserType
		}
	case *Influencer:
		if utype != InfluencerScope {
			return ErrInvalidUserType
		}
	default:
		return fmt.Errorf("unexpected type %T", su)
	}
	if err := su.Check(); err != nil {
		return err
	}
	return su.setToUser(a, u)
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
	if u.Type() == InvalidScope {
		return ErrInvalidUserType
	}
	// other checks?
	return nil
}

func (u *User) Store(a *Auth, tx *bolt.Tx) error {
	return misc.PutTxJson(tx, a.cfg.Bucket.User, u.ID, u)
}

func (u *User) StoreWithData(a *Auth, tx *bolt.Tx, data SpecUser) error {
	if err := u.UpdateData(a, data); err != nil {
		return err
	}
	return u.Store(a, tx)
}

func (u *User) Type() Scope {
	if u.Admin {
		return AdminScope
	}

	cnt, typ := 0, InvalidScope
	if u.AdAgency != nil {
		cnt++
		typ = AdAgencyScope
	}
	if u.TalentAgency != nil {
		cnt++
		typ = TalentAgencyScope
	}
	if u.Advertiser != nil {
		cnt++
		typ = AdvertiserScope
	}
	if u.InfluencerLoad != nil || u.Influencer != nil {
		cnt++
		typ = InfluencerScope
	}
	if cnt == 1 {
		return typ
	}
	return InvalidScope
}

func (a *Auth) CreateUserTx(tx *bolt.Tx, u *User, password string) (err error) {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = misc.TrimEmail(u.Email)
	if a.GetLoginTx(tx, u.Email) != nil {
		return ErrEmailExists
	}
	if err = u.Check(true); err != nil {
		return
	}

	if err != nil {
		return
	}

	u.CreatedAt = time.Now().UnixNano()
	u.UpdatedAt = u.CreatedAt
	u.Salt = hex.EncodeToString(misc.CreateToken(SaltLen))
	u.Status = true // always active on creation

	if password, err = HashPassword(password); err != nil {
		return
	}

	if u.ID, err = misc.GetNextIndex(tx, a.cfg.Bucket.User); err != nil {
		return
	}

	var suser SpecUser
	switch u.Type() {
	case AdvertiserScope:
		if u.Advertiser != nil {
			suser = u.Advertiser
		}
	case InfluencerScope:
		if u.InfluencerLoad != nil {
			suser = u.InfluencerLoad
		}
	case AdAgencyScope:
		if u.AdAgency != nil {
			suser = u.AdAgency
		}
	case TalentAgencyScope:
		if u.TalentAgency != nil {
			suser = u.TalentAgency
		}
	case AdminScope:
		goto SKIP
	}

	if suser == nil {
		return ErrInvalidRequest
	}

	if err = suser.setToUser(a, u); err != nil {
		return
	}

SKIP:
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
	switch userID {
	case AdminUserID, SwayOpsAdAgencyID, SwayOpsTalentAgencyID:
		return ErrInvalidID
	}
	user := a.GetUserTx(tx, userID)
	if user == nil {
		return ErrInvalidUserID
	}
	uid := []byte(userID)
	misc.GetBucket(tx, a.cfg.Bucket.User).Delete(uid)
	misc.GetBucket(tx, a.cfg.Bucket.Login).Delete([]byte(user.Email))
	switch user.Type() {
	case AdAgencyScope:
		return a.moveChildrenTx(tx, user.ID, SwayOpsAdAgencyID)
	case TalentAgencyScope:
		return a.moveChildrenTx(tx, user.ID, SwayOpsTalentAgencyID)
	}
	return nil
}

func (a *Auth) GetUserTx(tx *bolt.Tx, userID string) *User {
	var u User
	if misc.GetTxJson(tx, a.cfg.Bucket.User, userID, &u) == nil && len(u.Salt) > 0 {
		return &u
	}
	return nil
}
