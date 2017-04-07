package server

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

func setScrap(s *Server) gin.HandlerFunc {
	// Ingests a scrap and puts it into pool
	return func(c *gin.Context) {
		var (
			scraps []*influencer.Scrap
			err    error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&scraps); err != nil || len(scraps) == 0 {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if err := saveScraps(s, scraps); err != nil {
			c.JSON(500, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}

func optoutScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		scraps, _ := getAllScraps(s)
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
		scraps, err := getAllScraps(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, scraps)
	}
}

func getScrap(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		scrap, err := getScrapFromID(s, c.Param("id"))
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		c.JSON(200, scrap)
	}
}

func scrapStats(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSecureAdmin(c, s) {
			return
		}

		scraps, err := getAllScraps(s)
		if err != nil {
			c.JSON(400, misc.StatusErr(err.Error()))
			return
		}

		type ScrapStats struct {
			HasKeywords   int64 `json:"hasKeywords"`
			HasGeo        int64 `json:"hasGeo"`
			HasGender     int64 `json:"hasGender"`
			HasCategories int64 `json:"hasCategories"`
			Attributed    int64 `json:"attributed"`
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
		}

		c.JSON(200, stats)
	}
}
