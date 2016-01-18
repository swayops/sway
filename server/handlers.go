package server

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
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
			if ag.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Agency); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(ag); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Agency, ag.Id, b)
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
			v = tx.Bucket([]byte(s.Cfg.Bucket.Agency)).Get([]byte(c.Params.ByName("id")))
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
			tx.Bucket([]byte(s.Cfg.Bucket.Agency)).ForEach(func(k, v []byte) (err error) {
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

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Agency)).Get([]byte(agId)), &ag)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Agency, agId)
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
			if g.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Group); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}
			if b, err = json.Marshal(g); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Group, g.Id, b)
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
			v = tx.Bucket([]byte(s.Cfg.Bucket.Group)).Get([]byte(c.Params.ByName("id")))
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
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Group)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Group, gId)
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
			tx.Bucket([]byte(s.Cfg.Bucket.Group)).ForEach(func(k, v []byte) (err error) {
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
			tx.Bucket([]byte(s.Cfg.Bucket.Group)).ForEach(func(k, v []byte) (err error) {
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
			if adv.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Advertiser); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(adv); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Advertiser, adv.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(adv.Id))
	}
}

func getAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			g   common.Advertiser
		)

		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(c.Params.ByName("id")))
			return nil
		})

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g)
	}
}

func getAdvertisersByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAgency := c.Params.ByName("id")
		advertisers := make([]*common.Advertiser, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).ForEach(func(k, v []byte) (err error) {
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
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Advertiser)).Get([]byte(gId)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Advertiser, gId)
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

		// Save the Campaign
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if cmp.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Campaign); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			// Assuming each deal will be paying out max of $5
			// Lower this if you want less deals

			// The number of deals created is based on an avg
			// pay per deal value. These deals will be the pool
			// available.. no more.
			maxDeals := int(cmp.Budget / 5.0)
			deals := make(map[string]*common.Deal)
			for i := 0; i <= maxDeals; i++ {
				d := &common.Deal{
					Id:         misc.PseudoUUID(),
					CampaignId: cmp.Id,
				}
				deals[d.Id] = d
			}

			cmp.Deals = deals

			if b, err = json.Marshal(cmp); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(cmp.Id))
	}
}

func getCampaign(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			g   common.Campaign
		)

		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(c.Params.ByName("id")))
			return nil
		})

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g)
	}
}

func getCampaignsByAdvertiser(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAdv := c.Params.ByName("id")
		campaigns := make([]*common.Campaign, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).ForEach(func(k, v []byte) (err error) {
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
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Campaign, id)
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

///////// Influencers /////////
func putInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			load influencer.InfluencerLoad
			b    []byte
			err  error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&load); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		inf, err := influencer.New(
			load.TwitterId,
			load.InstagramId,
			load.FbId,
			load.YouTubeId,
			load.TumblrId,
			load.CategoryId,
			load.AgencyId,
			load.FloorPrice,
			s.Cfg)

		if err != nil {
			c.JSON(400, misc.StatusErr("Error when generating influencer data"))
			log.Println("Influencer generation error", err)
			return
		}

		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			if inf.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Influencer); err != nil {
				c.JSON(500, misc.StatusErr("Internal index error"))
				return
			}

			if b, err = json.Marshal(inf); err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b)
		}); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(inf.Id))
	}
}

func getInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			v   []byte
			err error
			g   influencer.Influencer
		)

		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("id")))
			return nil
		})

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g)
	}
}

func delInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Params.ByName("id")
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var g *influencer.Influencer
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(id)), &g)
			if err != nil {
				return err
			}

			err = misc.DelBucketBytes(tx, s.Cfg.Bucket.Influencer, id)
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

func getInfluencerByAgency(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetAg := c.Params.ByName("id")
		influencers := make([]*influencer.Influencer, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
				inf := &influencer.Influencer{}
				if err := json.Unmarshal(v, inf); err != nil {
					log.Println("error when unmarshalling influencer", string(v))
					return nil
				}
				if inf.AgencyId == targetAg {
					influencers = append(influencers, inf)
				}
				return
			})
			return nil
		})
		c.JSON(200, influencers)
	}
}

func getInfluencerByCategory(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		target := c.Params.ByName("id")
		influencers := make([]*influencer.Influencer, 0, 512)
		s.db.View(func(tx *bolt.Tx) error {
			tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
				inf := &influencer.Influencer{}
				if err := json.Unmarshal(v, inf); err != nil {
					log.Println("error when unmarshalling influencer", string(v))
					return nil
				}
				if inf.CategoryId == target {
					influencers = append(influencers, inf)
				}
				return
			})
			return nil
		})
		c.JSON(200, influencers)
	}
}

///////// Deals /////////
func getDealsByInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		influencerId := c.Params.ByName("influencerId")
		if len(influencerId) == 0 {
			c.JSON(500, misc.StatusErr("Influencer ID undefined"))
			return
		}

		deals := influencer.GetAvailableDeals(s.db, influencerId, "", s.Cfg)
		c.JSON(200, deals)
	}
}

func assignDeal(s *Server) gin.HandlerFunc {
	// Influencer accepting deal
	// Must pass in influencer ID and deal ID
	return func(c *gin.Context) {
		var (
			b []byte
		)

		dealId := c.Params.ByName("dealId")
		influencerId := c.Params.ByName("influencerId")
		campaignId := c.Params.ByName("campaignId")

		// Lets quickly make sure that this deal is still available
		// via our GetAvailableDeals func
		var found bool
		foundDeal := &common.Deal{}
		currentDeals := influencer.GetAvailableDeals(s.db, influencerId, dealId, s.Cfg)
		for _, deal := range currentDeals {
			if deal.Id == dealId && deal.CampaignId == campaignId && deal.Assigned == 0 && deal.InfluencerId == "" {
				found = true
				foundDeal = deal
			}
		}

		if !found {
			c.JSON(200, misc.StatusErr("Unforunately, the requested deal is no longer available!"))
			return
		}

		// Assign the deal & Save the Campaign
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var cmp *common.Campaign

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(campaignId)), &cmp)
			if err != nil {
				return err
			}

			if !cmp.Active {
				return errors.New("Campaign is no longer active")
			}

			foundDeal.InfluencerId = influencerId
			foundDeal.Assigned = int32(time.Now().Unix())
			cmp.Deals[dealId] = foundDeal

			// Append to the influencer's active deals
			var inf *influencer.Influencer
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(influencerId)), &inf)
			if err != nil {
				return err
			}

			if inf.ActiveDeals == nil || len(inf.ActiveDeals) == 0 {
				inf.ActiveDeals = []*common.Deal{}
			}
			inf.ActiveDeals = append(inf.ActiveDeals, foundDeal)

			// Save the Influencer
			if b, err = json.Marshal(inf); err != nil {
				return err
			}

			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
				return err
			}

			// Save the campaign
			if b, err = json.Marshal(cmp); err != nil {
				return err
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b)
		}); err != nil {
			c.JSON(200, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(dealId))
	}
}

// the deal isnt getting added so not showing up in htis handler
// put prints in assign
func getDealsAssignedToInfluencer(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			err error
			v   []byte
			g   influencer.Influencer
		)
		s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(c.Params.ByName("influencerId")))
			return nil
		})

		if err = json.Unmarshal(v, &g); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, g.ActiveDeals)
	}
}

func unassignDeal(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		dealId := c.Params.ByName("dealId")
		influencerId := c.Params.ByName("influencerId")
		campaignId := c.Params.ByName("campaignId")

		// Unssign the deal & Save the Campaign
		if err := s.db.Update(func(tx *bolt.Tx) (err error) {
			var (
				cmp *common.Campaign
				b   []byte
			)

			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Campaign)).Get([]byte(campaignId)), &cmp)
			if err != nil {
				return err
			}

			if deal, ok := cmp.Deals[dealId]; ok {
				deal.InfluencerId = ""
				deal.Assigned = 0
				deal.Completed = 0
				deal.Audited = 0
				deal.Platforms = make(map[string]float32)
				cmp.Deals[dealId] = deal
			}

			// Append to the influencer's cancellations and remove from active
			var inf *influencer.Influencer
			err = json.Unmarshal(tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).Get([]byte(influencerId)), &inf)
			if err != nil {
				return err
			}

			activeDeals := []*common.Deal{}
			for _, deal := range inf.ActiveDeals {
				if deal.Id != dealId {
					activeDeals = append(activeDeals, deal)
				}
			}

			inf.ActiveDeals = activeDeals
			inf.Cancellations += 1

			// Save the Influencer
			if b, err = json.Marshal(inf); err != nil {
				return err
			}

			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Influencer, inf.Id, b); err != nil {
				return err
			}

			// Save the campaign
			if b, err = json.Marshal(cmp); err != nil {
				return err
			}
			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Campaign, cmp.Id, b)
		}); err != nil {
			c.JSON(200, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(dealId))
	}
}

// func completeDeal(s *Server) gin.HandlerFunc {
// 	// Influencer marked this deal as completed
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK(""))
// 	}
// }

//// func getToBeAudited(s *Server) gin.HandlerFunc {
// 	// Get deals that are yet to be audited
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK(""))
// 	}
// }

// func auditSuccess(s *Server) gin.HandlerFunc {
// 	// The deal has been marked successful
// 	return func(c *gin.Context) {
// 		c.JSON(200, misc.StatusOK(""))
// 	}
// }
