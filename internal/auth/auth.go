package auth

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
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
	return &Auth{
		db:       db,
		cfg:      cfg,
		loginUrl: loginUrl,
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

func (a *Auth) CheckUser(c *gin.Context) {
	//c.Request.Cookie()
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
	return t.Expires > time.Now().UnixNano()
}

func (t *Token) Refresh(dur time.Duration) *Token {
	t.Expires = time.Now().Add(dur).UnixNano()
	return t
}

func GetSessionToken(tx *bolt.Tx, userID, oldToken string) (newToken string, err error) {
	var (
		parts = strings.Split(oldToken, "-")
		salt  []byte
	)
	if len(parts) == 3 {
		//if parts[0] != user
		salt = misc.CreateToken(8)
	}
}
