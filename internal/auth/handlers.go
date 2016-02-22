package auth

import (
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/misc"
)

func (a *Auth) SignInHandler(c *gin.Context) {
	var li struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"pass" form:"pass"`
	}
	if c.Bind(&li) != nil {
		c.JSON(http.StatusUnauthorized, misc.StatusErr(ErrInvalidRequest.Error()))
		return
	}
	var (
		login  *Login
		apiKey string
		tok    string
		err    error
	)
	a.db.Update(func(tx *bolt.Tx) (_ error) {
		if login, tok, err = a.SignInTx(tx, li.Email, li.Password); err != nil {
			return
		}
		u := a.GetUserTx(tx, login.UserId)
		if u == nil {
			err = ErrInvalidRequest // this should never ever ever happen
			return
		}
		apiKey = u.APIKey
		return
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, misc.StatusErr(err.Error()))
		return
	}

	mac := CreateMAC(login.Password, tok, apiKey)
	w := c.Writer
	setCookie(w, "token", tok, TokenAge)
	setCookie(w, "key", mac, TokenAge)
	c.JSON(200, misc.StatusOK(login.UserId))
}

func (a *Auth) SignupHandler(c *gin.Context) {
	var uwp struct { // UserWithPassword
		User
		Password  string `json:"pass"`
		Password2 string `json:"pass2"`
	}
	if err := c.BindJSON(&uwp); err != nil {
		c.JSON(400, misc.StatusErr(err.Error()))
		return
	}
	if uwp.Password != uwp.Password2 {
		c.JSON(400, misc.StatusErr(ErrPasswordMismatch.Error()))
		return
	}
	if len(uwp.Password) < 8 {
		c.JSON(400, misc.StatusErr(ErrShortPass.Error()))
		return
	}
	if err := a.db.Update(func(tx *bolt.Tx) error {
		return a.CreateUserTx(tx, &uwp.User, uwp.Password)
	}); err != nil {
		c.JSON(400, misc.StatusErr(err.Error()))
		return
	}
	c.JSON(200, misc.StatusOK(uwp.Id))
}
