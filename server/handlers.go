package server

import (
	"encoding/json"
	"log"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

const (
	AGENCY_BUCKET     = "agency"
	GROUP_BUCKET      = "group"
	ADVERTISER_BUCKET = "advertiser"
	CAMPAIGN_BUCKET   = "campaign"
	INFLUENCER_BUCKET = "influencer"
	DEAL_BUCKET       = "deal"
)

///////// Agencies /////////
func putAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ag  common.Agency
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&ag); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if ag.Id, err = misc.GetNextIndex(tx, AGENCY_BUCKET); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(ag); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, AGENCY_BUCKET, ag.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(ag.Id))
	}
}

func getAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			ag  common.Agency
		)

		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(AGENCY_BUCKET)).Get([]byte(c.Params.ByName("id")))
			return nil
		})

		if err = json.Unmarshal(v, &ag); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, ag)
	}
}

func getAllAgencies(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agenciesAll := make([]*common.Agency, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(AGENCY_BUCKET)).ForEach(func(k, v []byte) (err error) {
				ag := &common.Agency{}
				if err := json.Unmarshal(v, ag); err != nil {
					log.Println("error when unmarshalling agency", string(v))
					return nil
				}
				agenciesAll = append(agenciesAll, ag)
				return
			})
			return nil
		})
		c.JSON(200, agenciesAll)
	}
}

func delAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		agId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var ag *common.Agency

			err = json.Unmarshal(tx.Bucket([]byte(AGENCY_BUCKET)).Get([]byte(agId)), &ag)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, AGENCY_BUCKET, agId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(agId))
	}
}

///////// Groups /////////
func putGroup(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			g   common.Group
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&g); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if g.AgencyId == "" {
			c.JSON(400, misc.StatusErr("Must pass in valid agency ID"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if g.Id, err = misc.GetNextIndex(tx, GROUP_BUCKET); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(g); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			return misc.PutBucketBytes(tx, GROUP_BUCKET, g.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(g.Id))
	}
}

func getGroup(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			g   common.Group
		)

		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(GROUP_BUCKET)).Get([]byte(c.Params.ByName("id")))
			return nil
		})

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g)
	}
}

func delGroup(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		gId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Group
			err = json.Unmarshal(tx.Bucket([]byte(GROUP_BUCKET)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, GROUP_BUCKET, gId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(gId))
	}
}

func getGroupByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAgency := c.Params.ByName("id")
		groups := make([]*common.Group, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(GROUP_BUCKET)).ForEach(func(k, v []byte) (err error) {
				ag := &common.Group{}
				if err := json.Unmarshal(v, ag); err != nil {
					log.Println("error when unmarshalling group", string(v))
					return nil
				}
				if ag.AgencyId == targetAgency {
					groups = append(groups, ag)
				}
				return
			})
			return nil
		})
		c.JSON(200, groups)
	}
}

func getAllGroups(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups := make([]*common.Group, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(GROUP_BUCKET)).ForEach(func(k, v []byte) (err error) {
				ag := &common.Group{}
				if err := json.Unmarshal(v, ag); err != nil {
					log.Println("error when unmarshalling group", string(v))
					return nil
				}
				groups = append(groups, ag)
				return
			})
			return nil
		})
		c.JSON(200, groups)
	}
}

///////// Advertisers /////////
func putAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			adv common.Advertiser
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&adv); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if adv.Id, err = misc.GetNextIndex(tx, ADVERTISER_BUCKET); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(adv); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, ADVERTISER_BUCKET, adv.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(adv.Id))
	}
}

func getAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		gId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Advertiser
			err = json.Unmarshal(tx.Bucket([]byte(ADVERTISER_BUCKET)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, ADVERTISER_BUCKET, gId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(gId))
	}
}

func getAdvertisersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAgency := c.Params.ByName("id")
		advertisers := make([]*common.Advertiser, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(ADVERTISER_BUCKET)).ForEach(func(k, v []byte) (err error) {
				adv := &common.Advertiser{}
				if err := json.Unmarshal(v, adv); err != nil {
					log.Println("error when unmarshalling group", string(v))
					return nil
				}
				if adv.AgencyId == targetAgency {
					advertisers = append(advertisers, adv)
				}
				return
			})
			return nil
		})
		c.JSON(200, advertisers)
	}
}

func delAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		gId := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Advertiser
			err = json.Unmarshal(tx.Bucket([]byte(ADVERTISER_BUCKET)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, ADVERTISER_BUCKET, gId)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(gId))
	}
}

///////// Campaigns /////////
func putCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			cmp common.Campaign
			b   []byte
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&cmp); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if cmp.Id, err = misc.GetNextIndex(tx, CAMPAIGN_BUCKET); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(cmp); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, CAMPAIGN_BUCKET, cmp.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

func getCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Campaign
			err = json.Unmarshal(tx.Bucket([]byte(CAMPAIGN_BUCKET)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, CAMPAIGN_BUCKET, id)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}

func getCampaignsByAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAdv := c.Params.ByName("id")
		campaigns := make([]*common.Campaign, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(CAMPAIGN_BUCKET)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling group", string(v))
					return nil
				}
				if cmp.Adv == targetAdv {
					campaigns = append(campaigns, cmp)
				}
				return
			})
			return nil
		})
		c.JSON(200, campaigns)
	}
}

func delCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *common.Campaign
			err = json.Unmarshal(tx.Bucket([]byte(CAMPAIGN_BUCKET)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, CAMPAIGN_BUCKET, id)
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(id))
	}
}

///////// Deals /////////
// func putDeal(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("getallgrop"))
// 	}
// }

// func getDeal(s *Server) gin.HandlerFunc {
// 	// Replace with an advertiser json struct
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("getallgrop"))
// 	}
// }

// func delDeal(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("getallgrop"))
// 	}
// }

///////// Influencers /////////
// func putInfluencer(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("TBD"))
// 	}
// }

// func getInfluencer(s *Server) gin.HandlerFunc {
// 	// Replace with an advertiser json struct
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("TBD"))
// 	}
// }

// func delInfluencer(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("TBD"))
// 	}
// }

// func getInfluencerByAgency(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("TBD"))
// 	}
// }

// func getInfluencerByGroup(s *Server) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK("TBD"))
// 	}
// }
