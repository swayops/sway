package auth

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/missionMeteora/mandrill"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

const (
	TokenAge     = time.Hour * 6
	PermTokenAge = 100 * 365 * (time.Hour * 24) // 100 years should be long enough

	TokenLen     = 16 // it's actually 16 because CreateToken appends 8 bytes
	SaltLen      = 16
	ApiKeyHeader = `x-apikey`

	purgeFrequency = time.Hour * 24
)

type Auth struct {
	db       *bolt.DB
	cfg      *config.Config
	loginUrl string

	purgeTicker *time.Ticker
	ec          *mandrill.Client

	// In memory cache of influencers
	Influencers *influencer.Influencers
}

func New(db *bolt.DB, cfg *config.Config) *Auth {
	a := &Auth{
		db:          db,
		cfg:         cfg,
		ec:          cfg.MailClient(),
		Influencers: influencer.NewInfluencers(),
	}
	return a
}

func (a *Auth) PurgeInvalidTokens() {
	t := time.NewTicker(purgeFrequency)
	for {
		ts := time.Now()
		a.db.Update(func(tx *bolt.Tx) error {
			b := misc.GetBucket(tx, a.cfg.Bucket.Token)
			return b.ForEach(func(k, v []byte) error {
				var tok Token
				if json.Unmarshal(v, &tok) != nil || !tok.IsValid(ts) {
					b.Delete(k)
				}
				return nil
			})
		})

		<-t.C
	}

}

func (a *Auth) GetLoginTx(tx *bolt.Tx, email string) *Login {
	email = misc.TrimEmail(email)

	var l Login
	if misc.GetTxJson(tx, a.cfg.Bucket.Login, email, &l) == nil && l.UserID != "" {
		return &l
	}
	return nil
}

func (a *Auth) GetUserByEmailTx(tx *bolt.Tx, email string) *User {
	if l := a.GetLoginTx(tx, email); l != nil {
		return a.GetUserTx(tx, l.UserID)
	}
	return nil
}

type reqInfo struct {
	oldMac     string
	hashedPass string
	stoken     string
	user       *User
	isApiKey   bool
	subUser    string
}

func (a *Auth) getReqInfoTx(tx *bolt.Tx, req *http.Request) *reqInfo {
	var ri reqInfo
	if ri.stoken, ri.oldMac, ri.isApiKey = getCreds(req); ri.stoken == "" || ri.oldMac == "" {
		return nil
	}

	var token Token
	if misc.GetTxJson(tx, a.cfg.Bucket.Token, ri.stoken, &token) != nil || !token.IsValid(time.Now()) {
		return nil
	}
	if ri.user = a.GetUserTx(tx, token.UserID); ri.user == nil {
		return nil
	}
	if ri.isApiKey { // for api keys, we use the id as the password so the key wouldn't break if the user change it
		ri.hashedPass = ri.user.ID
		return &ri
	}
	email := ri.user.Email
	if ri.subUser = token.SubUser; ri.subUser != "" {
		email = ri.subUser
	}
	if l := a.GetLoginTx(tx, email); l != nil {
		ri.hashedPass = l.Password
	} else {
		return nil
	}
	return &ri
}

func (a *Auth) SignInTx(tx *bolt.Tx, email, pass string, perm bool) (l *Login, stok string, err error) {
	if l = a.GetLoginTx(tx, email); l == nil {
		return nil, "", ErrInvalidEmail
	}
	if !CheckPassword(l.Password, pass) {
		return nil, "", ErrInvalidPass
	}

	age := TokenAge
	if perm {
		age = PermTokenAge
	}

	stok = hex.EncodeToString(misc.CreateToken(TokenLen - 8))
	ntok := &Token{UserID: l.UserID, Expires: time.Now().Add(age).UnixNano()}
	if l.IsSubUser {
		ntok.SubUser = email
	}
	err = misc.PutTxJson(tx, a.cfg.Bucket.Token, stok, ntok)
	return
}

func (a *Auth) SignOutTx(tx *bolt.Tx, stok string) error {
	return misc.GetBucket(tx, a.cfg.Bucket.Token).Delete([]byte(stok))
}

func (a *Auth) SignIn(email, pass string, perm bool) (l *Login, stok string, err error) {
	a.db.Update(func(tx *bolt.Tx) error {
		l, stok, err = a.SignInTx(tx, email, pass, perm)
		return nil
	})
	return
}

func (a *Auth) GetUsersByTypeTx(tx *bolt.Tx, typ Scope, fn func(u *User) error) error {
	return tx.Bucket([]byte(a.cfg.Bucket.User)).ForEach(func(k []byte, v []byte) error {
		var u User
		if json.Unmarshal(v, &u) == nil {
			if typ == AllScopes || typ == u.Type() {
				if err := fn(&u); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Auth) AddSubUsersTx(tx *bolt.Tx, userID, email, pass string) (err error) {
	if email = misc.TrimEmail(email); a.GetLoginTx(tx, email) != nil {
		return ErrUserExists
	}

	if pass, err = HashPassword(pass); err != nil {
		return
	}

	return misc.PutTxJson(tx, a.cfg.Bucket.Login, email, &Login{
		UserID:    userID,
		Password:  pass,
		IsSubUser: true,
	})
}

func (a *Auth) NukeTing() (err error) {
	if err := a.db.Update(func(tx *bolt.Tx) error {
		if err := misc.DelBucketBytes(tx, a.cfg.Bucket.Login, "dhillar@tucows.com"); err != nil {
			return err
		}

		if err := misc.DelBucketBytes(tx, a.cfg.Bucket.User, "476"); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (a *Auth) ListSubUsersTx(tx *bolt.Tx, userID string) (out []string) {
	misc.GetBucket(tx, a.cfg.Bucket.Login).ForEach(func(k []byte, v []byte) error {
		var l Login
		if json.Unmarshal(v, &l) == nil && l.UserID == userID && l.IsSubUser {
			out = append(out, string(k)) // user's email
		}
		return nil
	})

	return
}

type Token struct {
	UserID  string `json:"userId"`
	Email   string `json:"email"`
	Expires int64  `json:"expires"`

	// if IsSubUser is true it means this is a sub-user under and advertiser.
	// this is only used by the UI
	SubUser string `json:"subUser,omitempty"`
}

func (t *Token) IsValid(ts time.Time) bool {
	return (t.UserID != "" || t.Email != "") && t.Expires == -1 || t.Expires > ts.UnixNano()
}

func (t *Token) Refresh(dur time.Duration) *Token {
	if t.Expires != -1 {
		t.Expires = time.Now().Add(dur).UnixNano()
	}
	return t
}

func (a *Auth) refreshToken(stok string, dur time.Duration) {
	a.db.Update(func(tx *bolt.Tx) (_ error) {
		var token Token
		if misc.GetTxJson(tx, a.cfg.Bucket.Token, stok, &token) != nil || !token.IsValid(time.Now()) {
			return
		}
		return misc.PutTxJson(tx, a.cfg.Bucket.Token, stok, token.Refresh(dur))
	})
}

func (a *Auth) moveChildrenTx(tx *bolt.Tx, oldID, newID string) error {
	return tx.Bucket([]byte(a.cfg.Bucket.User)).ForEach(func(k []byte, v []byte) error {
		var u User
		if json.Unmarshal(v, &u) == nil && u.ParentID == oldID {
			u.ParentID = newID
			return u.Store(a, tx)
		}
		return nil
	})
}
