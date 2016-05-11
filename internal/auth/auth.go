package auth

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/missionMeteora/mandrill"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const (
	TokenAge     = time.Hour * 6
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
}

func New(db *bolt.DB, cfg *config.Config) *Auth {
	a := &Auth{
		db:  db,
		cfg: cfg,
		ec:  cfg.MailClient(),
	}
	return a
}

func (a *Auth) PurgeInvalidTokens() {
	for {
		a.db.Update(func(tx *bolt.Tx) error {
			b := misc.GetBucket(tx, a.cfg.AuthBucket.Token)
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
	email = misc.TrimEmail(email)

	var l Login
	if misc.GetTxJson(tx, a.cfg.AuthBucket.Login, email, &l) == nil && l.UserId != "" {
		return &l
	}
	return nil
}

func (a *Auth) GetUserByEmailTx(tx *bolt.Tx, email string) *User {
	if l := a.GetLoginTx(tx, email); l != nil {
		return a.GetUserTx(tx, l.UserId)
	}
	return nil
}

type reqInfo struct {
	oldMac     string
	hashedPass string
	stoken     string
	user       *User
	isApiKey   bool
}

func (a *Auth) getReqInfoTx(tx *bolt.Tx, req *http.Request) *reqInfo {
	var ri reqInfo
	if ri.stoken, ri.oldMac, ri.isApiKey = getCreds(req); ri.stoken == "" || ri.oldMac == "" {
		return nil
	}

	var token Token
	if misc.GetTxJson(tx, a.cfg.AuthBucket.Token, ri.stoken, &token) != nil || !token.IsValid(time.Now()) {
		return nil
	}
	if ri.user = a.GetUserTx(tx, token.UserId); ri.user == nil {
		return nil
	}
	if ri.isApiKey { // for api keys, we use the id as the password so the key wouldn't break if the user change it
		ri.hashedPass = ri.user.Id
		return &ri
	}
	if l := a.GetLoginTx(tx, ri.user.Email); l != nil {
		ri.hashedPass = l.Password
	} else {
		return nil
	}
	return &ri
}

func (a *Auth) SignInTx(tx *bolt.Tx, email, pass string) (l *Login, stok string, err error) {
	if l = a.GetLoginTx(tx, email); l == nil {
		return nil, "", ErrInvalidEmail
	}
	if !CheckPassword(l.Password, pass) {
		return nil, "", ErrInvalidPass
	}
	stok = hex.EncodeToString(misc.CreateToken(TokenLen - 8))
	ntok := &Token{UserId: l.UserId, Expires: time.Now().Add(TokenAge).UnixNano()}
	err = misc.PutTxJson(tx, a.cfg.AuthBucket.Token, stok, ntok)
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
	Email   string `json:"email"`
	Expires int64  `json:"expires"`
}

func (t *Token) IsValid(ts time.Time) bool {
	return (t.UserId != "" || t.Email != "") && t.Expires == -1 || t.Expires > ts.UnixNano()
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
		if misc.GetTxJson(tx, a.cfg.AuthBucket.Token, stok, &token) != nil || !token.IsValid(time.Now()) {
			return
		}
		return misc.PutTxJson(tx, a.cfg.AuthBucket.Token, stok, token.Refresh(dur))
	})
}
