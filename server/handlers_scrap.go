package server

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

func setScrap(s *Server) gin.HandlerFunc {
	// Ingests a scrap and puts it into pool
	return func(c *gin.Context) {
		var (
			scraps []influencer.Scrap
			err    error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&scraps); err != nil || len(scraps) == 0 {
			misc.WriteJSON(c, 400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err := saveScraps(s, scraps); err != nil {
			misc.WriteJSON(c, 500, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

func optoutScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		scraps := s.Scraps.GetStore()
		for _, sc := range scraps {
			if sc.EmailAddress == email {
				sc.Ignore = true
				saveScrap(s, sc)
			}
		}

		c.String(200, "You have successfully been opted out. You may now close this window.")

	}
}

func getScraps(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		scraps := s.Scraps.GetStore()
		misc.WriteJSON(c, 200, scraps)
	}
}

func getScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		scrap, _ := s.Scraps.Get(c.Param("id"))
		misc.WriteJSON(c, 200, scrap)
	}
}

func scrapStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		scraps := s.Scraps.GetStore()

		type ScrapStats struct {
			HasKeywords   int64 `json:"hasKeywords"`
			HasGeo        int64 `json:"hasGeo"`
			HasGender     int64 `json:"hasGender"`
			HasCategories int64 `json:"hasCategories"`
			Attributed    int64 `json:"attributed"`
			Touched       int64 `json:"touched"`
			Total         int   `json:"total"`
		}

		stats := ScrapStats{Total: len(scraps)}
		for _, sc := range scraps {
			if sc.Updated > 0 {
				stats.Attributed += 1
			}

			if len(sc.Keywords) > 0 {
				stats.HasKeywords += 1
			}

			if len(sc.Categories) > 0 {
				stats.HasCategories += 1
			}

			if sc.Geo != nil {
				stats.HasGeo += 1
			}

			if sc.Male || sc.Female {
				stats.HasGender += 1
			}

			if len(sc.SentEmails) > 0 {
				stats.Touched += 1
			}
		}

		misc.WriteJSON(c, 200, stats)
	}
}

func getScrapByHandle(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		scraps := s.Scraps.GetStore()

		handle := c.Param("id")
		platform := c.Param("platform")
		email := c.Query("email")

		var found influencer.Scrap
		for _, sc := range scraps {
			if email != "" && strings.EqualFold(sc.EmailAddress, email) {
				found = sc
				break
			}

			switch platform {
			case "instagram":
				if sc.InstaData != nil && sc.InstaData.UserName == handle {
					found = sc
					break
				}
			case "facebook":
				if sc.FBData != nil && sc.FBData.Id == handle {
					found = sc
					break
				}
			case "youtube":
				if sc.YTData != nil && sc.YTData.UserName == handle {
					found = sc
					break
				}
			case "twitter":
				if sc.TWData != nil && sc.TWData.Id == handle {
					found = sc
					break
				}
			}
		}

		misc.WriteJSON(c, 200, found)
	}
}
