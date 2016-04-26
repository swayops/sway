package auth

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

func (a *Auth) VerifyUser(allowAnon bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		var ri *reqInfo
		a.db.View(func(tx *bolt.Tx) error {
			ri = a.getReqInfoTx(tx, c.Request)
			return nil
		})
		if ri == nil && allowAnon {
			return
		}
		w, r := c.Writer, c.Request
		if ri == nil || !VerifyMac(ri.oldMac, ri.hashedPass, ri.stoken, ri.user.Salt) {
			if a.loginUrl != "" && r.Method == "GET" && r.Header.Get("X-Requested-With") == "" {
				c.Redirect(302, a.loginUrl)
			} else {
				misc.AbortWithErr(c, 401, ErrUnauthorized)
			}
			return
		}
		c.Set(gin.AuthUserKey, ri.user)
		if !ri.isApiKey {
			refreshCookie(w, r, "token", TokenAge)
			refreshCookie(w, r, "key", TokenAge)
			a.refreshToken(ri.stoken, TokenAge)
		}
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

func (a *Auth) SignUpHandler(c *gin.Context) {
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

const resetPasswordUrl = "%s/resetPassword/%s"

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
	}{a.cfg.Sandbox, fmt.Sprintf(resetPasswordUrl, a.cfg.ServerURL, stok)}

	email := templates.ResetPassword.Render(tmplData)
	if resp, err := a.ec.SendMessage(email, "Password Reset Request", req.Email, u.Name,
		[]string{"reset password"}); err != nil || len(resp) != 1 || resp[0].RejectReason != "" {
		log.Printf("%v: %+v", err, resp)
		misc.AbortWithErr(c, 500, ErrUnexpected)
		return
	}
	c.JSON(200, misc.StatusOK(""))
}

func (a *Auth) ResetHandler(c *gin.Context) {
	var req struct {
		Token     string `json:"token"`
		Email     string `json:"email"`
		Password  string `json:"pass"`
		Password1 string `json:"pass1"`
	}

	if c.BindJSON(&req) != nil || req.Email == "" || req.Token == "" {
		misc.AbortWithErr(c, 400, ErrInvalidRequest)
		return
	}

	if req.Password != req.Password1 {
		misc.AbortWithErr(c, 400, ErrPasswordMismatch)
		return
	}
	if err := a.db.Update(func(tx *bolt.Tx) error {
		return a.ResetPasswordTx(tx, req.Token, req.Email, req.Password)
	}); err != nil {
		misc.AbortWithErr(c, 400, err)
		return
	}
	c.JSON(200, misc.StatusOK(""))
}

// this returns a perma API key for the logged in user, the key gets invalidated if the user changes their password
// or passes ?renew=true
func (a *Auth) APIKeyHandler(c *gin.Context) {
	u := GetCtxUser(c)
	if u.APIKey == "" || c.Query("renew") == "true" {
		stok := hex.EncodeToString(misc.CreateToken(TokenLen - 8))
		ntok := &Token{UserId: u.Id, Expires: -1}
		var pass string
		if err := a.db.Update(func(tx *bolt.Tx) error {
			l := a.GetLoginTx(tx, u.Email)
			if l == nil {
				return ErrInvalidEmail
			}
			pass = l.Password
			return misc.PutTxJson(tx, a.cfg.AuthBucket.Token, stok, ntok)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}
		mac := CreateMAC(pass, stok, u.Salt)
		if err := a.db.Update(func(tx *bolt.Tx) error {
			u.APIKey = stok + mac
			return misc.PutTxJson(tx, a.cfg.AuthBucket.User, u.Id, u)
		}); err != nil {
			misc.AbortWithErr(c, 400, err)
			return
		}

	}
	msg := misc.StatusOK(u.Id)
	msg["key"] = u.APIKey
	c.JSON(200, msg)
}
