package auth

import (
	"encoding/hex"
	"encoding/json"
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
	UserID    string `json:"userID"`
	Password  string `json:"password"`
	IsSubUser bool   `json:"isSubUser,omitempty"`
}

type User struct {
	ID            string `json:"id"`
	ParentID      string `json:"parentId,omitempty"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Address       string `json:"address,omitempty"`
	ImageURL      string `json:"imageUrl,omitempty"`
	CoverImageURL string `json:"coverImageUrl,omitempty"`
	Status        bool   `json:"status,omitempty"`
	CreatedAt     int64  `json:"createdAt,omitempty"`
	UpdatedAt     int64  `json:"updatedAt,omitempty"`
	APIKey        string `json:"apiKeys,omitempty"`
	Salt          string `json:"salt,omitempty"`
	Admin         bool   `json:"admin,omitempty"`
	//	Data      json.RawMessage `json:"Data,omitempty"`

	AdAgency     *AdAgency     `json:"adAgency,omitempty"`
	TalentAgency *TalentAgency `json:"talentAgency,omitempty"`
	Advertiser   *Advertiser   `json:"advertiser,omitempty"`
	Influencer   *Influencer   `json:"inf,omitempty"`

	//special hack, the gods will look down upon us and spit
	InfluencerLoad *InfluencerLoad `json:"influencer,omitempty"`

	SubUser string `json:"subUser,omitempty"`
}

type signupUser struct {
	User
	Password  string `json:"pass"`
	Password2 string `json:"pass2"`
}

// Trim returns a browser-safe version of the User, mainly hiding salt, and maybe possibly apiKeys
func (u *User) Trim() *User {
	ou := *u
	ou.Salt = ""
	return &ou
}

// Update fills the updatable fields in the struct, fields like Created and ID should never be blindly set.
func (u *User) Update(o *User) *User {
	if o.Name != "" {
		u.Name = o.Name
	}
	// if o.Email = misc.TrimEmail(o.Email); o.Email != "" {
	// 	u.Email = o.Email
	// }
	if o.Phone != "" {
		u.Phone = o.Phone
	}

	if o.Address != "" {
		u.Address = o.Address
	}

	if o.ImageURL != "" {
		u.ImageURL = o.ImageURL
	}

	if o.CoverImageURL != "" {
		u.CoverImageURL = o.CoverImageURL
	}

	u.Status = o.Status
	u.UpdatedAt = time.Now().Unix()
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

	switch u.Type() {
	case InvalidScope:
		return ErrInvalidUserType
	case InfluencerScope:
		if len(strings.Split(u.Name, " ")) < 2 {
			return ErrInvalidName
		}
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

	u.CreatedAt = time.Now().Unix()
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

	if err = suser.Check(); err != nil {
		return
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

func (a *Auth) ChangeLoginTx(tx *bolt.Tx, userID, newEmail string) error {
	newEmail = misc.TrimEmail(newEmail)
	lB := misc.GetBucket(tx, a.cfg.Bucket.Login)
	if len(lB.Get([]byte(newEmail))) > 0 {
		return ErrEmailExists
	}
	u := a.GetUserTx(tx, userID)
	if u == nil {
		return ErrInvalidID
	}

	// the login info, contains the user id and password
	loginInfo := a.GetLoginTx(tx, u.Email)
	if loginInfo == nil || loginInfo.UserID != userID {
		return ErrInvalidEmail
	}

	lB.Delete([]byte(u.Email))

	u.Email = newEmail
	if err := misc.PutTxJson(tx, a.cfg.Bucket.Login, u.Email, loginInfo); err != nil {
		return err
	}

	return u.Store(a, tx)
}

func (a *Auth) GetUser(userID string) (u *User) {
	a.db.View(func(tx *bolt.Tx) error {
		u = a.GetUserTx(tx, userID)
		return nil
	})
	return
}

func (a *Auth) GetChildCountsTx(tx *bolt.Tx, uids ...string) map[string]int {
	counts := make(map[string]int, len(uids))
	for _, uid := range uids {
		counts[uid] = 0
	}

	misc.GetBucket(tx, a.cfg.Bucket.User).ForEach(func(_, v []byte) error {
		var u struct {
			ParentID string `json:"parentID"`
		}
		json.Unmarshal(v, &u)
		if _, ok := counts[u.ParentID]; ok {
			counts[u.ParentID]++
		}
		return nil
	})

	return counts
}

func (a *Auth) GetChildCounts(uids ...string) (out map[string]int) {

	a.db.View(func(tx *bolt.Tx) error {
		out = a.GetChildCountsTx(tx, uids...)
		return nil
	})
	return
}
