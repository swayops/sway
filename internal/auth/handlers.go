package auth

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
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
				misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
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
		misc.AbortWithErr(c, 405, ErrUnauthorized)
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
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		if u.Type == AdminScope { // admin owns everything
			return
		}
		var ok bool
		switch itemType {
		case AdvertiserItem:
			switch u.Type {
			case AdvertiserScope:
				ok = u.ID == itemID
			case AdAgencyScope:
				ok = u.Owns(itemID)
			}
		case CampaignItem:
			cmp := common.GetCampaign(itemID, a.db, a.cfg)
			switch u.Type {
			case AdvertiserScope:
				ok = u.ID == cmp.AdvertiserId
			case AdAgencyScope:
				ok = u.Owns(cmp.AdvertiserId)
			}

		case InfluencerItem:
			switch u.Type {
			case InfluencerScope:
				ok = u.ID == itemID
			case TalentAgencyScope:
				ok = u.Owns(itemID)
			}

		}
		if !ok {
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		}
	}
}

func (a *Auth) SignOutHandler(c *gin.Context) {
	tok := getCookie(c.Request, "token")
	if tok == "" {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	a.db.Update(func(tx *bolt.Tx) (_ error) {
		return a.SignOutTx(tx, tok)
	})
	w := c.Writer
	deleteCookie(w, "token")
	deleteCookie(w, "key")
	c.JSON(200, misc.StatusOK(""))
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
		u := a.GetUserTx(tx, login.UserID)
		if u == nil {
			err = ErrInvalidRequest // this should never ever ever happen
			return
		}
		salt = u.Salt
		return
	})

	if err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}

	mac := CreateMAC(login.Password, tok, salt)
	w := c.Writer
	setCookie(w, "token", tok, TokenAge)
	setCookie(w, "key", mac, TokenAge)
	c.JSON(200, misc.StatusOK(login.UserID))
}

// SignUpHelper handles common user sign up operations, returns a *User.
// if nil is returned, it means it failed and already returned an http error
func (a *Auth) signUpHelper(c *gin.Context, sup *signupUser) (_ bool) {
	if sup.Type == AdminScope {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	curUser := GetCtxUser(c)
	if curUser != nil {
		if !curUser.Type.CanCreate(sup.Type) {
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		sup.ParentID = curUser.ID
	} else if sup.Type == AdvertiserScope {
		sup.ParentID = SwayOpsAdAgencyID
	} else if sup.Type == InfluencerScope {
		sup.ParentID = SwayOpsTalentAgencyID
	} else {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	if sup.Password != sup.Password2 {
		misc.AbortWithErr(c, http.StatusBadRequest, ErrPasswordMismatch)
		return
	}

	if len(sup.Password) < 8 {
		misc.AbortWithErr(c, http.StatusBadRequest, ErrShortPass)
		return
	}

	if err := a.db.Update(func(tx *bolt.Tx) error {
		if err := a.CreateUserTx(tx, &sup.User, sup.Password); err != nil {
			return err
		}
		if curUser != nil {
			return curUser.AddChild(sup.ID).Store(a, tx)
		}
		return nil
	}); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	return true
}

func (a *Auth) SignUpHandler(c *gin.Context) {
	var sup signupUser
	if err := c.Bind(&sup); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	if a.signUpHelper(c, &sup) {
		c.JSON(200, misc.StatusOK(sup.ID))
	}
}

const resetPasswordUrl = "%s/resetPassword/%s"

func (a *Auth) ReqResetHandler(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if c.BindJSON(&req) != nil || len(req.Email) == 0 {
		misc.AbortWithErr(c, http.StatusBadRequest, ErrInvalidRequest)
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
		misc.AbortWithErr(c, http.StatusBadRequest, ErrInvalidRequest)
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
		misc.AbortWithErr(c, http.StatusBadRequest, ErrInvalidRequest)
		return
	}

	if req.Password != req.Password1 {
		misc.AbortWithErr(c, http.StatusBadRequest, ErrPasswordMismatch)
		return
	}
	if err := a.db.Update(func(tx *bolt.Tx) error {
		return a.ResetPasswordTx(tx, req.Token, req.Email, req.Password)
	}); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(200, misc.StatusOK(""))
}

// this returns a perma API key for the logged in user, the user can pass ?renew=true to generate a new key.
func (a *Auth) APIKeyHandler(c *gin.Context) {
	u := GetCtxUser(c)
	if u == nil {
		var li struct {
			Email    string `json:"email" form:"email"`
			Password string `json:"pass" form:"pass"`
		}
		if err := c.Bind(&li); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
		if err := a.db.View(func(tx *bolt.Tx) error {
			l := a.GetLoginTx(tx, li.Email)
			if l == nil {
				return ErrInvalidEmail
			}
			if !CheckPassword(l.Password, li.Password) {
				return ErrInvalidPass
			}
			if u = a.GetUserTx(tx, l.UserID); u == nil { // should never ever happen
				return ErrInvalidID
			}
			return nil
		}); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
	}

	if u.APIKey == "" || c.Query("renew") == "true" {
		stok := hex.EncodeToString(misc.CreateToken(TokenLen - 8))
		ntok := &Token{UserID: u.ID, Expires: -1}
		if err := a.db.Update(func(tx *bolt.Tx) error {
			l := a.GetLoginTx(tx, u.Email)
			if l == nil {
				return ErrInvalidEmail
			}
			return misc.PutTxJson(tx, a.cfg.Bucket.Token, stok, ntok)
		}); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
		mac := CreateMAC(u.ID, stok, u.Salt)
		if err := a.db.Update(func(tx *bolt.Tx) error {
			log.Println(u.APIKey, len(u.APIKey))
			if len(u.APIKey) == 96 { // delete the old key
				tx.Bucket([]byte(a.cfg.Bucket.Token)).Delete([]byte(u.APIKey[:32]))
			}
			u.APIKey = stok + mac
			return misc.PutTxJson(tx, a.cfg.Bucket.User, u.ID, u)
		}); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
	}

	msg := misc.StatusOK(u.ID)
	msg["key"] = u.APIKey
	c.JSON(200, msg)
}
