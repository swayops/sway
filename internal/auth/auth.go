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
	TokenAge = time.Hour * 6
)

type Auth struct {
	db       *bolt.DB
	cfg      *config.Config
	loginUrl string
}

func New(db *bolt.DB, cfg *config.Config, loginUrl string) *Auth {
	a := &Auth{
		db:       db,
		cfg:      cfg,
		loginUrl: loginUrl,
	}
	go a.purgeInvalidTokens()
	return a
}

func (a *Auth) purgeInvalidTokens() {
	a.db.Update(func(tx *bolt.Tx) error {
		var tok Token
		b := misc.GetBucket(tx, a.cfg.Bucket.Token)
		return b.ForEach(func(k, v []byte) error {
			if json.Unmarshal(v, &tok) != nil || !tok.Valid() {
				b.Delete(k)
			}
			return nil
		})
	})
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
	if misc.GetTxJson(tx, a.cfg.Bucket.Login, userId, &u) == nil && len(u.APIKey) > 0 {
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

func (a *Auth) getReqInfoTx(tx *bolt.Tx, req *http.Request) (oldKey, userId, hashedPass, stoken, apiKey string) {
	var (
		token     Token
		user      *User
		login     *Login
		stok, key = getCookie(req, "token"), getCookie(req, "key")
	)
	if len(stok) == 0 || len(key) == 0 {
		parts := strings.Split(req.Header.Get("auth"), "|")
		if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
			return
		}
		stok, key = parts[0], parts[1]
	}
	if misc.GetTxJson(tx, a.cfg.Bucket.Token, stok, &token) != nil || !token.Valid() {
		return
	}
	if user = a.GetUserTx(tx, token.UserId); user == nil {
		return
	}
	if login = a.GetLoginTx(tx, user.Email); login == nil {
		return
	}
	return key, user.Id, login.Password, stok, user.APIKey
}

func (a *Auth) SignInTx(tx *bolt.Tx, email, pass string) (l *Login, stok string, err error) {
	if l = a.GetLoginTx(tx, email); l == nil {
		return nil, "", ErrInvalidEmail
	}
	if !CheckPassword(l.Password, pass) {
		return nil, "", ErrInvalidPass
	}
	stok = hex.EncodeToString(misc.CreateToken(8))
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

func (t *Token) Valid() bool {
	return t.UserId != "" && t.Expires > time.Now().UnixNano()
}

func (t *Token) Refresh(dur time.Duration) *Token {
	t.Expires = time.Now().Add(dur).UnixNano()
	return t
}
