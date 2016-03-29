package auth

import (
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/templates"
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

// CheckScopes returns a gin handler that checks user access against the provided ScopeMap
func (a *Auth) CheckScopes(sm ScopeMap) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := GetCtxUser(c); u != nil && sm.HasAccess(u.Type, c.Request.Method) {
			return
		}
		misc.AbortWithErr(c, 401, ErrUnauthorized)
	}
}

//	CheckOwnership returns a handler that checks the ownership of an item.
//	params:
//		- itemType (ex CampaignItem)
//		- paramName from the route (ex :id)
func (a *Auth) CheckOwnership(itemType ItemType, paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, itemID := GetCtxUser(c), c.Param(paramName)
		if u == nil || itemID == "" {
			misc.AbortWithErr(c, 400, ErrInvalidRequest)
			return
		}
		if u.Type == Admin { // admin owns everything
			return
		}
		var ok bool
		a.db.View(func(tx *bolt.Tx) error {
			oid := a.GetOwnerTx(tx, itemType, itemID)
			if ok = oid == u.Id; ok {
				return nil
			}
			// a parent owns all his children's assists, what a cruel cruel world.
			for ou := a.GetUserTx(tx, oid); ou != nil && u.ParentId != ""; u = a.GetUserTx(tx, u.ParentId) {
				if ok = u.Id == ou.ParentId; ok {
					break
				}
			}
			return nil
		})
		if !ok {
			misc.AbortWithErr(c, 401, ErrUnauthorized)
		}
	}
}

func (a *Auth) SignInHandler(c *gin.Context) {
	var li struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"pass" form:"pass"`
	}
	if err := c.Bind(&li); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
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
	if uwp.Type == "" {
		uwp.Type = Advertiser
	}
	currentUser := GetCtxUser(c)
	if currentUser != nil {
		if !currentUser.Type.CanCreate(uwp.Type) {
			misc.AbortWithErr(c, 401, ErrUnauthorized)
			return
		}
		uwp.ParentId = currentUser.Id
	} else if uwp.Type != Advertiser {
		misc.AbortWithErr(c, 401, ErrUnauthorized)
		return
	} else {
		uwp.ParentId = SwayOpsAgencyId
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

const resetPasswordUrl = "%s%s/resetPassword/%s"

func (a *Auth) ReqResetHandler(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if c.BindJSON(&req) != nil || len(req.Email) == 0 {
		misc.AbortWithErr(c, 400, ErrInvalidRequest)
		return
	}
	var (
		u    *User
		stok string
		err  error
	)
	a.db.Update(func(tx *bolt.Tx) error {
		u, stok, err = a.RequestResetPasswordTx(tx, req.Email)
		return nil
	})
	if err != nil {
		misc.AbortWithErr(c, 400, ErrInvalidRequest)
		return
	}
	tmplData := struct {
		Sandbox bool
		URL     string
	}{a.cfg.Sandbox, fmt.Sprintf(resetPasswordUrl, a.cfg.ServerURL, a.cfg.APIPath, stok)}

	email := templates.ResetPassword.Render(tmplData)
	if resp, err := a.cfg.MailClient().SendMessage(email, "Password Reset Request", req.Email, u.Name, []string{"reset password"}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		log.Printf("%v: %+v", err, resp)
		misc.AbortWithErr(c, 500, ErrUnexpected)
		return
	}
	c.JSON(200, misc.StatusOK(""))
}
