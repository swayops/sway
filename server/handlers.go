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
	agency_bucket     = "agency"
	group_bucket      = "group"
	advertiser_bucket = "advertiser"
	campaign_bucket   = "campaign"
	influencer_bucket = "influencer"
	deal_bucket       = "deal"
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
			if ag.Id, err = misc.GetNextIndex(tx, agency_bucket); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(ag); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, agency_bucket, ag.Id, b)
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
			v = tx.Bucket([]byte(agency_bucket)).Get([]byte(c.Params.ByName("id")))
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
			tx.Bucket([]byte(agency_bucket)).ForEach(func(k, v []byte) (err error) {
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

			err = json.Unmarshal(tx.Bucket([]byte(agency_bucket)).Get([]byte(agId)), &ag)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, agency_bucket, agId)
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
			if g.Id, err = misc.GetNextIndex(tx, group_bucket); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(g); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			return misc.PutBucketBytes(tx, group_bucket, g.Id, b)
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
			v = tx.Bucket([]byte(group_bucket)).Get([]byte(c.Params.ByName("id")))
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
			err = json.Unmarshal(tx.Bucket([]byte(group_bucket)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, group_bucket, gId)
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
			tx.Bucket([]byte(group_bucket)).ForEach(func(k, v []byte) (err error) {
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
			tx.Bucket([]byte(group_bucket)).ForEach(func(k, v []byte) (err error) {
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
			if adv.Id, err = misc.GetNextIndex(tx, advertiser_bucket); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(adv); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, advertiser_bucket, adv.Id, b)
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
			err = json.Unmarshal(tx.Bucket([]byte(advertiser_bucket)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, advertiser_bucket, gId)
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
			tx.Bucket([]byte(advertiser_bucket)).ForEach(func(k, v []byte) (err error) {
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
			err = json.Unmarshal(tx.Bucket([]byte(advertiser_bucket)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, advertiser_bucket, gId)
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
			if cmp.Id, err = misc.GetNextIndex(tx, campaign_bucket); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(cmp); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, campaign_bucket, cmp.Id, b)
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
			err = json.Unmarshal(tx.Bucket([]byte(campaign_bucket)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, campaign_bucket, id)
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
			tx.Bucket([]byte(campaign_bucket)).ForEach(func(k, v []byte) (err error) {
				cmp := &common.Campaign{}
				if err := json.Unmarshal(v, cmp); err != nil {
					log.Println("error when unmarshalling group", string(v))
					return nil
				}
				if cmp.AdvertiserId == targetAdv {
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
			err = json.Unmarshal(tx.Bucket([]byte(campaign_bucket)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, campaign_bucket, id)
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
