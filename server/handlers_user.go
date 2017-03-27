package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/misc"
)

///////// Talent Agencies ///////////
func putTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "talentAgency")
	}
}

func getTalentAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ag *auth.TalentAgency
		s.db.View(func(tx *bolt.Tx) error {
			ag = s.auth.GetTalentAgencyTx(tx, c.Param("id"))
			return nil
		})
		if ag == nil {
			misc.AbortWithErr(c, 400, auth.ErrInvalidAgencyID)
			return
		}
		c.JSON(200, ag)
	}
}

func getAllTalentAgencies(s *Server) gin.HandlerFunc {
	type userWithCounts struct {
		*auth.User
		SubCount int64 `json:"subCount"`
	}
	return func(c *gin.Context) {
		var (
			all []*userWithCounts
		)

		s.db.View(func(tx *bolt.Tx) error {
			s.auth.GetUsersByTypeTx(tx, auth.TalentAgencyScope, func(u *auth.User) error {
				if u.TalentAgency != nil {
					all = append(all, &userWithCounts{u.Trim(), s.auth.Influencers.GetCount(u.ID)})
				}
				return nil
			})
			return nil
		})
		c.JSON(200, all)
	}
}

///////// Ad Agencies /////////
func putAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "adAgency")
	}
}

func getAdAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ag *auth.AdAgency
		s.db.View(func(tx *bolt.Tx) error {
			ag = s.auth.GetAdAgencyTx(tx, c.Param("id"))
			return nil
		})
		if ag == nil {
			misc.AbortWithErr(c, 400, auth.ErrInvalidAgencyID)
			return
		}
		c.JSON(200, ag)
	}
}

func getAllAdAgencies(s *Server) gin.HandlerFunc {
	type userWithCounts struct {
		*auth.User
		SubCount int `json:"subCount"`
	}
	return func(c *gin.Context) {
		var (
			all    []*userWithCounts
			counts map[string]int
			uids   []string
		)

		s.db.View(func(tx *bolt.Tx) error {
			s.auth.GetUsersByTypeTx(tx, auth.AdAgencyScope, func(u *auth.User) error {
				if u.AdAgency != nil { // should always be true, but just in case
					all = append(all, &userWithCounts{u.Trim(), 0})
					uids = append(uids, u.ID)
				}
				return nil
			})
			counts = s.auth.GetChildCountsTx(tx, uids...)
			return nil
		})

		for _, u := range all {
			u.SubCount = counts[u.ID]
		}
		c.JSON(200, all)
	}
}

func putAdmin(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "admin")

	}
}

func userProfile(srv *Server) gin.HandlerFunc {
	checkAdAgency := srv.auth.CheckOwnership(auth.AdAgencyItem, "id")
	checkTalentAgency := srv.auth.CheckOwnership(auth.TalentAgencyItem, "id")

	return func(c *gin.Context) {
		cu := auth.GetCtxUser(c)
		id := c.Param("id")

		if id == "" || id == cu.ID {
			goto SKIP
		}

		switch {
		case cu.Admin:
		case cu.AdAgency != nil:
			checkAdAgency(c)
		case cu.TalentAgency != nil:
			checkTalentAgency(c)
		default:
			misc.AbortWithErr(c, http.StatusUnauthorized, auth.ErrUnauthorized)
		}
		if c.IsAborted() {
			return
		}

		if cu = srv.auth.GetUser(id); cu == nil {
			misc.AbortWithErr(c, http.StatusNotFound, auth.ErrInvalidUserID)
		}

	SKIP:
		cu = cu.Trim()

		if cu.Advertiser == nil { // return the user if it isn't an advertiser
			c.JSON(200, cu)
			return
		}

		var advWithCampaigns struct {
			*auth.User
			HasCampaigns bool `json:"hasCmps"`
		}

		advWithCampaigns.User = cu

		srv.db.View(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(srv.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp struct {
					AdvertiserId string `json:"advertiserId"`
				}
				if json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				// if the campaign's adv id is the same as this user it means he has at least one cmp
				// set the flag and break the foreach early
				if cmp.AdvertiserId == cu.ID {
					advWithCampaigns.HasCampaigns = true
					return io.EOF
				}
				return
			})
		})

		c.JSON(200, &advWithCampaigns)
	}
}
