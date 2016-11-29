package auth

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/sharpspring"
	"github.com/swayops/sway/internal/templates"
	"github.com/swayops/sway/misc"
)

func (a *Auth) VerifyUser(allowAnon bool) func(c *gin.Context) {
	domain := a.cfg.Domain
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
			misc.RefreshCookie(w, r, domain, "token", TokenAge)
			misc.RefreshCookie(w, r, domain, "key", TokenAge)
			a.refreshToken(ri.stoken, TokenAge)
		}
	}
}

// CheckScopes returns a gin handler that checks user access against the provided ScopeMap
func (a *Auth) CheckScopes(sm ScopeMap) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := GetCtxUser(c); u != nil && sm.HasAccess(u.Type(), c.Request.Method) {
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
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		if u.Admin { // admin owns everything
			return
		}
		var (
			ok    bool
			utype = u.Type()
		)
		if utype == InvalidScope {
			goto EXIT
		}

		switch itemType {
		case AdAgencyItem:
			if ok = u.ID == itemID; !ok {
				adv := a.GetAdvertiser(itemID)
				ok = adv != nil && adv.AgencyID == u.ID
			}

		case TalentAgencyItem:
			if ok = u.ID == itemID; !ok {
				inf := a.GetUser(itemID)
				ok = inf != nil && inf.Influencer != nil && inf.ParentID == u.ID
			}

		case AdvertiserItem:
			switch utype {
			case AdvertiserScope:
				ok = u.ID == itemID
			case AdAgencyScope:
				adv := a.GetAdvertiser(itemID)
				ok = adv != nil && adv.AgencyID == u.ID
			}

		case CampaignItem:
			cmp := common.GetCampaign(itemID, a.db, a.cfg)
			switch utype {
			case AdvertiserScope:
				ok = u.ID == cmp.AdvertiserId
			case AdAgencyScope:
				adv := a.GetAdvertiser(cmp.AdvertiserId)
				ok = adv != nil && adv.AgencyID == u.ID
			}

		case InfluencerItem:
			switch utype {
			case InfluencerScope:
				ok = u.ID == itemID
			case TalentAgencyScope:
				inf := a.GetInfluencer(itemID)
				ok = inf != nil && inf.AgencyId == u.ID
			}

		default:
			log.Println("unexpected:", itemType)
		}
	EXIT:
		if !ok {
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		}
	}
}

func (a *Auth) SignOutHandler(c *gin.Context) {
	tok := misc.GetCookie(c.Request, "token")
	if tok == "" {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	a.db.Update(func(tx *bolt.Tx) (_ error) {
		return a.SignOutTx(tx, tok)
	})
	w, domain := c.Writer, a.cfg.Domain
	misc.DeleteCookie(w, domain, "token", !a.cfg.Sandbox)
	misc.DeleteCookie(w, domain, "key", !a.cfg.Sandbox)
	c.JSON(200, misc.StatusOK(""))
}

func signInHelper(a *Auth, c *gin.Context, email, pass string) (_ bool) {
	var (
		login *Login
		salt  string
		tok   string
		err   error
	)

	a.db.Update(func(tx *bolt.Tx) (_ error) {
		if login, tok, err = a.SignInTx(tx, email, pass); err != nil {
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
	w, domain := c.Writer, a.cfg.Domain
	misc.SetCookie(w, domain, "token", tok, !a.cfg.Sandbox, TokenAge)
	misc.SetCookie(w, domain, "key", mac, !a.cfg.Sandbox, TokenAge)
	c.JSON(200, misc.StatusOK(login.UserID))
	return true
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
	signInHelper(a, c, li.Email, li.Password)
}

// SignUpHelper handles common user sign up operations, returns a *User.
// if nil is returned, it means it failed and already returned an http error
func (a *Auth) signUpHelper(c *gin.Context, sup *signupUser) (_ bool) {
	if sup.Admin {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	typ := sup.Type()
	if typ == InvalidScope {
		misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	curUser := GetCtxUser(c)
	if curUser != nil {
		if !curUser.Type().CanCreate(sup.Type()) {
			misc.AbortWithErr(c, http.StatusUnauthorized, ErrUnauthorized)
			return
		}

		if curUser.Admin {
			// if the admin is creating a user they either have to specify the parent id or
			// it will be automatically set to the sway agency
			if sup.ParentID == "" {
				switch typ {
				case AdvertiserScope:
					if sup.Advertiser != nil && sup.Advertiser.AgencyID != "" {
						sup.ParentID = sup.Advertiser.AgencyID
					} else {
						sup.ParentID = SwayOpsAdAgencyID
					}
				case InfluencerScope:
					sup.ParentID = SwayOpsTalentAgencyID
				default:
					sup.ParentID = curUser.ID
				}
			}
		} else {
			sup.ParentID = curUser.ID
		}
	} else if typ == AdvertiserScope {
		sup.ParentID = SwayOpsAdAgencyID
	} else if typ == InfluencerScope {
		sup.ParentID = SwayOpsTalentAgencyID

		if sup.InfluencerLoad != nil && sup.InfluencerLoad.IP == "" {
			sup.InfluencerLoad.IP = c.ClientIP()
		}

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
		return nil
	}); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}

	var ssType string
	if sup.Advertiser != nil {
		ssType = sharpspring.AdvList
	} else if sup.Influencer != nil {
		ssType = sharpspring.InfList
	}

	if ssType != "" && !a.cfg.Sandbox {
		go func(qs string) {
			err := sharpspring.CreateLead(a.cfg, ssType, sup.ID, sup.ParentID, sup.Name, sup.Email, qs)
			log.Printf("error creating lead: %v", err)
		}(c.Request.URL.RawQuery)
	}

	return true
}

func (a *Auth) SignUpHandler(c *gin.Context) {
	var sup signupUser
	if err := c.Bind(&sup); err != nil {
		misc.AbortWithErr(c, http.StatusBadRequest, err)
		return
	}
	if !a.signUpHelper(c, &sup) {
		return
	}
	if c.Query("autologin") != "" {
		signInHelper(a, c, sup.Email, sup.Password)
	} else {
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
	}{a.cfg.Sandbox, fmt.Sprintf(resetPasswordUrl, a.cfg.DashURL, stok)}

	log.Println("resetPassword request:", tmplData.URL)

	email := templates.ResetPassword.Render(tmplData)
	if resp, err := a.ec.SendMessage(email, "Password Reset Request", req.Email, u.Name,
		[]string{"reset password"}); err != nil {
		log.Printf("error sending reset email to %s: %+v: %v", email, resp, err)
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
