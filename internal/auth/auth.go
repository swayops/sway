package auth

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	TokenAge       = time.Hour * 6
	TokenLen       = 16
	TokenStringLen = TokenLen * 2
	SaltLen        = 16
	SaltStringLen  = TokenLen * 2
	MacLen         = 32
	MacStringLen   = MacLen * 2
	ApiKeyHeader   = `x-apikey`

	purgeFrequency = time.Hour * 24
)

type Auth struct {
	db       *bolt.DB
	cfg      *config.Config
	loginUrl string

	purgeTicker *time.Ticker
}

func New(db *bolt.DB, cfg *config.Config) *Auth {
	a := &Auth{
		db:  db,
		cfg: cfg,
	}
	go a.purgeInvalidTokens()
	return a
}

func (a *Auth) purgeInvalidTokens() {
	for {
		a.db.Update(func(tx *bolt.Tx) error {
			b := misc.GetBucket(tx, a.cfg.Bucket.Token)
			ts := time.Now()
			return b.ForEach(func(k, v []byte) error {
				var tok Token
				if json.Unmarshal(v, &tok) != nil || !tok.IsValid(ts) {
					b.Delete(k)
				}
				return nil
			})
		})

		time.Sleep(purgeFrequency)
	}

}
func (a *Auth) GetLoginTx(tx *bolt.Tx, email string) *Login {
	email = strings.ToLower(strings.TrimSpace(email))

	var l Login
	if misc.GetTxJson(tx, a.cfg.Bucket.Login, email, &l) == nil && l.UserId != "" {
		return &l
	}
	return nil
}

func (a *Auth) GetUserTx(tx *bolt.Tx, userId string) *User {
	var u User
	if misc.GetTxJson(tx, a.cfg.Bucket.Login, userId, &u) == nil && len(u.Salt) > 0 {
		return &u
	}
	return nil
}

func (a *Auth) GetUserByEmailTx(tx *bolt.Tx, email string) *User {
	if l := a.GetLoginTx(tx, email); l != nil {
		return a.GetUserTx(tx, l.UserId)
	}
	return nil
}

func (a *Auth) getReqInfoTx(tx *bolt.Tx, req *http.Request) (oldMac, hashedPass, stoken string, user *User, isApiKey bool) {
	if stoken, oldMac, isApiKey = getCreds(req); len(stoken) == 0 || len(oldMac) == 0 {
		return
	}

	var token Token
	if misc.GetTxJson(tx, a.cfg.Bucket.Token, stoken, &token) != nil || !token.IsValid(time.Now()) {
		return
	}
	if user = a.GetUserTx(tx, token.UserId); user == nil {
		return
	}
	if l := a.GetLoginTx(tx, user.Email); l != nil {
		hashedPass = l.Password
	} else {
		user = nil
	}
	return
}

func (a *Auth) SignInTx(tx *bolt.Tx, email, pass string) (l *Login, stok string, err error) {
	if l = a.GetLoginTx(tx, email); l == nil {
		return nil, "", ErrInvalidEmail
	}
	if !CheckPassword(l.Password, pass) {
		return nil, "", ErrInvalidPass
	}
	stok = hex.EncodeToString(misc.CreateToken(TokenLen - 8)) // -8 because CreateToken adds 8 bytes
	err = misc.PutTxJson(tx, a.cfg.Bucket.Token, stok, &Token{l.UserId, time.Now().Add(TokenAge).UnixNano()})
	return
}

func (a *Auth) SignIn(email, pass string) (l *Login, stok string, err error) {
	a.db.Update(func(tx *bolt.Tx) error {
		l, stok, err = a.SignInTx(tx, email, pass)
		return nil
	})
	return
}

type Token struct {
	UserId  string `json:"userId"`
	Expires int64  `json:"expires"`
}

func (t *Token) IsValid(ts time.Time) bool {
	return t.UserId != "" && t.Expires == -1 || t.Expires > ts.UnixNano()
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
