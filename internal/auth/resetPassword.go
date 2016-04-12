package auth

import (
	"encoding/hex"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/misc"
)

const (
	ResetTokenLen    = 16
	ResetTokenMaxAge = time.Hour * 48
)

func (a *Auth) RequestResetPasswordTx(tx *bolt.Tx, email string) (*User, string, error) {
	u := a.GetUserByEmailTx(tx, email)
	if u == nil {
		return nil, "", ErrInvalidEmail
	}
	stok := hex.EncodeToString(misc.CreateToken(ResetTokenLen - 8))
	tokVal := Token{Email: misc.TrimEmail(email), Expires: time.Now().Add(ResetTokenMaxAge).UnixNano()}
	return u, stok, misc.PutTxJson(tx, a.cfg.AuthBucket.Token, stok, tokVal)
}

func (a *Auth) ChangePasswordTx(tx *bolt.Tx, email, oldPassword, newPassword string, force bool) error {
	l := a.GetLoginTx(tx, email)
	if l == nil {
		return ErrInvalidEmail
	}
	if !force && !CheckPassword(l.Password, oldPassword) {
		return ErrInvalidPass
	}
	var err error
	if l.Password, err = HashPassword(newPassword); err != nil {
		return err
	}
	return misc.PutTxJson(tx, a.cfg.AuthBucket.Login, misc.TrimEmail(email), l)
}

func (a *Auth) ResetPasswordTx(tx *bolt.Tx, stok, email, newPassword string) error {
	email = misc.TrimEmail(email)
	var tok Token
	if misc.GetTxJson(tx, a.cfg.AuthBucket.Token, stok, &tok) != nil || !tok.IsValid(time.Now()) {
		return ErrInvalidRequest
	}
	if tok.Email != email {
		return ErrInvalidRequest
	}
	tx.Bucket([]byte(a.cfg.AuthBucket.Token)).Delete([]byte(stok))
	return a.ChangePasswordTx(tx, email, "", newPassword, true)
}
