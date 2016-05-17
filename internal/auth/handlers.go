package auth

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
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
			misc.AbortWithErr(c, http.StatusBadRequest, ErrInvalidRequest)
			return
		}
		if u.Type == AdminScope { // admin owns everything
			return
		}
		var ok bool
		switch itemType {
		case AdAgencyItem, TalentAgencyItem:
			ok = u.OwnsItem(itemType, itemID)
		case AdvertiserItem:
			a.db.View(func(tx *bolt.Tx) error {
				adv := a.GetAdvertiserTx(tx, itemID)
				ok = adv != nil && u.OwnsItem(AdAgencyItem, adv.AgencyId)
				return nil
			})
		case CampaignItem:
			a.db.View(func(tx *bolt.Tx) error {
				var cmp common.Campaign
				if misc.GetTxJson(tx, a.cfg.Bucket.Campaign, itemID, &cmp) != nil {
					return nil
				}
				adv := a.GetAdvertiserTx(tx, cmp.AdvertiserId)
				log.Println(cmp.AdvertiserId, adv, u.Id, u.Items)
				ok = adv != nil && u.OwnsItem(AdAgencyItem, adv.AgencyId)
				return nil
			})
		case InfluencerItem:
			a.db.View(func(tx *bolt.Tx) error {
				var inf influencer.Influencer
				if misc.GetTxJson(tx, a.cfg.Bucket.Influencer, itemID, &inf) != nil {
					return nil
				}
				ok = inf.AgencyId != "" && u.OwnsItem(TalentAgencyItem, inf.AgencyId)
				return nil
			})
		}
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

// SignUpHelper handles common user sign up operations, returns a *User.
// if nil is returned, it means it failed and already returned an http error
func (a *Auth) SignUpHelper(c *gin.Context, sup *SignupUser, defaultScope Scope) (_ bool) {
	if sup.Type == "" {
		sup.Type = defaultScope
	}
	currentUser := GetCtxUser(c)
	if currentUser != nil {
		if !currentUser.Type.CanCreate(sup.Type) {
			misc.AbortWithErr(c, 401, ErrUnauthorized)
			return
		}
		sup.ParentId = currentUser.Id
	} else if sup.Type != AdvertiserScope {
		misc.AbortWithErr(c, 401, ErrUnauthorized)
		return
	} else {
		sup.ParentId = SwayOpsAgencyId
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
		return a.CreateUserTx(tx, &sup.User, sup.Password)
	}); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	return true
}

func (a *Auth) SignUpHandler(c *gin.Context) {
	var sup SignupUser
	if err := c.Bind(&sup); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	if a.SignUpHelper(c, &sup, AdvertiserScope) {
		c.JSON(200, misc.StatusOK(sup.Id))
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
			if u = a.GetUserTx(tx, l.UserId); u == nil { // should never ever happen
				return ErrInvalidId
			}
			return nil
		}); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
	}

	if u.APIKey == "" || c.Query("renew") == "true" {
		stok := hex.EncodeToString(misc.CreateToken(TokenLen - 8))
		ntok := &Token{UserId: u.Id, Expires: -1}
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
		mac := CreateMAC(u.Id, stok, u.Salt)
		if err := a.db.Update(func(tx *bolt.Tx) error {
			log.Println(u.APIKey, len(u.APIKey))
			if len(u.APIKey) == 96 { // delete the old key
				tx.Bucket([]byte(a.cfg.Bucket.Token)).Delete([]byte(u.APIKey[:32]))
			}
			u.APIKey = stok + mac
			return misc.PutTxJson(tx, a.cfg.Bucket.User, u.Id, u)
		}); err != nil {
			misc.AbortWithErr(c, http.StatusBadRequest, err)
			return
		}
	}

	msg := misc.StatusOK(u.Id)
	msg["key"] = u.APIKey
	c.JSON(200, msg)
}
