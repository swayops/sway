package auth

import (
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/misc"
)

func (a *Auth) VerifyUser(c *gin.Context) {
	var (
		oldMac, hashedPass, stoken string

		isApiKey bool
		user     *User
	)
	a.db.View(func(tx *bolt.Tx) error {
		oldMac, hashedPass, stoken, user, isApiKey = a.getReqInfoTx(tx, c.Request)
		return nil
	})
	w, r := c.Writer, c.Request
	if len(hashedPass) == 0 || !VerifyMac(oldMac, hashedPass, stoken, user.Salt) {
		if a.loginUrl != "" && r.Method == "GET" && r.Header.Get("X-Requested-With") == "" {
			c.Redirect(302, a.loginUrl)
		} else {
			misc.AbortWithErr(c, 401, ErrUnauthorized)
		}
		return
	}
	c.Set(gin.AuthUserKey, user)
	if !isApiKey {
		refreshCookie(w, r, "token", TokenAge)
		refreshCookie(w, r, "key", TokenAge)
		a.refreshToken(stoken, TokenAge)
	}
}

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
		login *Login
		salt  string
		tok   string
		err   error
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
		salt = u.Salt
		return
	})

	if err != nil {
		misc.AbortWithErr(c, 401, err)
		return
	}

	mac := CreateMAC(login.Password, tok, salt)
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
		misc.AbortWithErr(c, 400, err)
		return
	}
	if uwp.Password != uwp.Password2 {
		misc.AbortWithErr(c, 400, ErrPasswordMismatch)
		return
	}
	if len(uwp.Password) < 8 {
		misc.AbortWithErr(c, 400, ErrShortPass)
		return
	}
	if err := a.db.Update(func(tx *bolt.Tx) error {
		return a.CreateUserTx(tx, &uwp.User, uwp.Password)
	}); err != nil {
		misc.AbortWithErr(c, 400, err)
		return
	}
	c.JSON(200, misc.StatusOK(uwp.Id))
}
