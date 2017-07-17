package server

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

var ErrID = errors.New("Invalid click URL")

func click(s *Server) gin.HandlerFunc {
	domain := s.Cfg.Domain
	return func(c *gin.Context) {
		var (
			id = c.Param("id")
			v  []byte
		)

		id = strings.TrimPrefix(id, "/")

		if err := s.db.View(func(tx *bolt.Tx) error {
			v = tx.Bucket([]byte(s.Cfg.Bucket.URL)).Get([]byte(id))
			return nil
		}); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrID.Error()))
			return
		}

		parts := strings.Split(string(v), "::")
		if len(parts) != 2 {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrID.Error()))
			return
		}

		campaignId := parts[0]
		dealId := parts[1]

		cmp := common.GetCampaign(campaignId, s.db, s.Cfg)
		if cmp == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrCampaign.Error()))
			return
		}

		foundDeal, ok := cmp.Deals[dealId]
		if !ok || foundDeal == nil {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrDealNotFound.Error()))
			return
		}

		if campaignId == "21" {
			// Hack!
			foundDeal.Link = "https://www.amazon.com/gp/product/B01C2EFBZU?th=1"
		}

		if foundDeal.Link == "" {
			misc.WriteJSON(c, 500, misc.StatusErr(ErrDealNotFound.Error()))
			return
		}

		infId := foundDeal.InfluencerId
		// Stored as a comma separated list of dealIDs satisfied
		cookie := misc.GetCookie(c.Request, "click")
		// if strings.Contains(cookie, foundDeal.Id) && c.Query("dbg") != "1" {
		// 	// This user has already clicked once for this deal!
		// 	c.Redirect(302, foundDeal.Link)
		// 	return
		// }

		// ip := c.ClientIP()
		// ua := c.Request.UserAgent()
		// // Has this user's IP and UA combination been seen before?
		// if s.ClickSet.Exists(ip, ua) && !s.Cfg.Sandbox {
		// 	// This user has already clicked once for this deal!
		// 	c.Redirect(302, foundDeal.Link)
		// 	return
		// }

		inf, ok := s.auth.Influencers.Get(infId)
		if !ok {
			log.Println("Influencer not found for click!", infId, campaignId)
			c.Redirect(302, foundDeal.Link)
			return
		}

		var (
			added bool
			uuid  string
		)

		// Lets see if they have an old UUID
		if cookie != "" {
			parts := strings.Split(cookie, ":")
			if len(parts) == 2 {
				uuid = parts[0]
			}
		}

		if uuid == "" {
			// New user.. assign them a UUID!
			uuid = misc.PseudoUUID()
		}

		// Lets search in completed deals first!
		for _, infDeal := range inf.CompletedDeals {
			if foundDeal.Id == infDeal.Id {
				infDeal.Click(uuid)
				added = true
				break
			}
		}

		if !added {
			// Ok lets check active deals if deal wasn't found in completed!
			for _, infDeal := range inf.ActiveDeals {
				if foundDeal.Id == infDeal.Id {
					infDeal.Click(uuid)
					added = true
					break
				}
			}
		}

		// SAVE!
		// Also saves influencers!
		if added {
			// s.ClickSet.Set(ip, ua)

			if err := saveAllDeals(s, inf); err != nil {
				c.Redirect(302, foundDeal.Link)
				return
			}

			if cookie != "" {
				cookie += "," + foundDeal.Id
			} else {
				cookie += uuid + ":" + foundDeal.Id
			}

			// One click per 30 days allowed per deal!
			// Format of prev clicks:
			// UUID:DEALID1,DEALID2,DEALID3
			misc.SetCookie(c.Writer, domain, "click", cookie, !s.Cfg.Sandbox, 24*30*time.Hour)

			if err := s.Cfg.Loggers.Log("clicks", map[string]interface{}{
				"dealId":     foundDeal.Id,
				"campaignId": foundDeal.CampaignId,
				"uuid":       uuid,
				"cookie":     cookie,
			}); err != nil {
				log.Println("Failed to log click!", foundDeal.Id, foundDeal.CampaignId)
			}

			if !s.Cfg.Sandbox {
				go func() {
					if err := misc.Ping(s.Cfg.ConverterURL + "click/" + uuid + "/" + foundDeal.Id + "/" + foundDeal.CampaignId + "/" + foundDeal.AdvertiserId); err != nil {
						s.Alert("Failed to ping converter for "+foundDeal.AdvertiserId, err)
					}
				}()
			}
		}

		c.Redirect(302, foundDeal.Link)
	}
}

func getTotalClicks(s *Server) gin.HandlerFunc {
	// Return all clicks that happened in the last X hours
	return func(c *gin.Context) {
		hours, _ := strconv.Atoi(c.Param("hours"))
		if hours == 0 {
			c.String(400, "Invalid hours value")
			return
		}

		total := make(map[string]int)

		for _, inf := range s.auth.Influencers.GetAll() {
			for _, deal := range inf.CompletedDeals {
				for _, stats := range deal.Reporting {
					for _, cl := range stats.ApprovedClicks {
						if misc.WithinLast(cl.TS, int32(hours)) {
							total[deal.CampaignId] += 1
						}
					}
				}
			}
		}

		misc.WriteJSON(c, 200, total)
	}
}

type Click struct {
	TS           int32  `json:"ts,omitempty"`
	DealID       string `json:"dealID"`
	CampaignID   string `json:"campaignID"`
	AdvertiserID string `json:"advertiserID"`
}

func exportClicks(s *Server) gin.HandlerFunc {
	// Return all clicks that happened in the last X hours
	// {"UUID": [{DealID: DEAL ID, campaign id: 123, ts: 123321 }]}
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.Param("days"))
		if days == 0 {
			c.String(400, "Invalid days value")
			return
		}

		total := make(map[string][]*Click)

		for _, cmp := range s.Campaigns.GetStore() {
			for _, deal := range cmp.Deals {
				if !deal.IsComplete() && !deal.IsActive() {
					continue
				}

				// We now have a completed or active deal! Lets get all it's pending
				// AND approved clicks

				for _, stats := range deal.Reporting {
					for _, cl := range stats.PendingClicks {
						if cl.UUID != "" && misc.WithinLast(cl.TS, int32(days*24)) {
							total[cl.UUID] = append(total[cl.UUID], &Click{cl.TS, deal.Id, deal.CampaignId, deal.AdvertiserId})
						}
					}

					for _, cl := range stats.ApprovedClicks {
						if cl.UUID != "" && misc.WithinLast(cl.TS, int32(days*24)) {
							total[cl.UUID] = append(total[cl.UUID], &Click{cl.TS, deal.Id, deal.CampaignId, deal.AdvertiserId})
						}
					}
				}

			}
		}

		misc.WriteJSON(c, 200, total)
	}
}
