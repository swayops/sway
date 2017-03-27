package server

import (
	"encoding/json"
	"log"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		saveUserHelper(s, c, "advertiser")
	}
}

func getAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		adv := s.auth.GetAdvertiser(c.Param("id"))
		if adv == nil {
			c.JSON(500, misc.StatusErr("Internal error"))
			return
		}
		c.JSON(200, adv)
	}
}

func getAdvertisersByAgency(s *Server) gin.HandlerFunc {
	type advWithCounts struct {
		*auth.User
		NumCampaigns int `json:"numCmps"`
	}
	return func(c *gin.Context) {
		var (
			targetAgency = c.Param("id")
			advertisers  []*advWithCounts
			counts       = map[string]int{}
		)
		if err := s.db.View(func(tx *bolt.Tx) error {
			if u := s.auth.GetUserTx(tx, targetAgency); u == nil || u.Type() != auth.AdAgencyScope {
				return auth.ErrInvalidUserID
			}
			s.auth.GetUsersByTypeTx(tx, auth.AdvertiserScope, func(u *auth.User) error {
				if u.Advertiser != nil && u.ParentID == targetAgency {
					advertisers = append(advertisers, &advWithCounts{u.Trim(), 0})
					counts[u.ID] = 0
				}
				return nil
			})
			return tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				var cmp struct {
					AdvertiserId string `json:"advertiserId"`
				}
				if json.Unmarshal(v, &cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return nil
				}
				if _, ok := counts[cmp.AdvertiserId]; ok {
					counts[cmp.AdvertiserId]++
				}
				return
			})
		}); err != nil {
			misc.AbortWithErr(c, 404, err)
			return
		}
		for _, adv := range advertisers {
			adv.NumCampaigns = counts[adv.ID]
		}
		c.JSON(200, advertisers)
	}
}

func advertiserBan(s *Server) gin.HandlerFunc {
	// Retrieves all completed deals by advertiser
	return func(c *gin.Context) {
		id := c.Param("id")
		adv := s.auth.GetAdvertiser(id)
		if adv == nil {
			c.JSON(500, misc.StatusErr("Please provide a valid advertiser"))
			return
		}

		infId := c.Param("influencerId")
		if infId == "" {
			c.JSON(500, misc.StatusErr("Please provide a valid influencer"))
			return
		}

		user := auth.GetCtxUser(c)
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			if id != user.ID {
				user = s.auth.GetUserTx(tx, id)
			}
			if user == nil {
				return auth.ErrInvalidID
			}

			if len(adv.Blacklist) == 0 {
				adv.Blacklist = make(map[string]bool)
			}
			adv.Blacklist[infId] = true
			return user.StoreWithData(s.auth, tx, adv)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		// Copy the new blacklist to all campaigns under the advertiser!
		if err := s.db.Update(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling campaign", string(v))
					return err
				}

				if cmp.AdvertiserId == adv.ID {
					cmp.Blacklist = adv.Blacklist
				}

				if err = saveCampaign(tx, cmp, s); err != nil {
					log.Println("Error saving campaign for adv blacklist", err)
					return err
				}

				return nil
			})
			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}
