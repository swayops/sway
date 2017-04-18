package server

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

func audience(s *Server) gin.HandlerFunc {
	// Ingests a list of emails, audience name and saves (or overwrites existing ID)
	return func(c *gin.Context) {
		var (
			aud common.Audience
			err error
		)

		defer c.Request.Body.Close()
		if err = json.NewDecoder(c.Request.Body).Decode(&aud); err != nil {
			c.JSON(400, misc.StatusErr("Error unmarshalling request body"))
			return
		}

		if len(aud.Members) == 0 {
			c.JSON(400, misc.StatusErr("Please provide a valid audience list"))
			return
		}
		aud.Members = common.TrimEmails(aud.Members)

		if aud.Name == "" {
			c.JSON(400, misc.StatusErr("Please provide a valid audience name"))
			return
		}

		// If an ID is not passed in we assume a new audience is being made
		if aud.Id == "" {
			if err = s.db.Update(func(tx *bolt.Tx) (err error) { // have to get an id early for saveImage
				aud.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Audience)
				return
			}); err != nil {
				c.JSON(500, misc.StatusErr(err.Error()))
				return
			}
		}

		if aud.ImageData != "" {
			if !strings.HasPrefix(aud.ImageData, "data:image/") {
				c.JSON(400, misc.StatusErr("Please provide a valid audience image"))
				return
			}

			// NOTE FOR Ahmed: Change min size and height as per ur liking
			filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Audience, aud.Id), aud.ImageData, aud.Id, "", 750, 389)
			if err != nil {
				c.JSON(400, misc.StatusErr(err.Error()))
				return
			}

			aud.ImageURL, aud.ImageData = getImageUrl(s, s.Cfg.Bucket.Audience, "dash", filename, false), ""
		}

		// Save the Audience
		if err = s.db.Update(func(tx *bolt.Tx) (err error) {
			var (
				b []byte
			)

			if b, err = json.Marshal(aud); err != nil {
				return err
			}

			s.Audiences.SetAudience(aud.Id, &aud)

			return misc.PutBucketBytes(tx, s.Cfg.Bucket.Audience, aud.Id, b)
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		c.JSON(200, misc.StatusOK(aud.Id))
	}
}

func getAudiences(s *Server) gin.HandlerFunc {
	// Optional "ID" param to filter to one audience, otherwise it returns
	// all audiences
	return func(c *gin.Context) {
		c.JSON(200, s.Audiences.GetStore(c.Param("id")))
	}
}

func delAudience(s *Server) gin.HandlerFunc {
	// Optional "ID" param to filter to one audience, otherwise it returns
	// all audiences
	return func(c *gin.Context) {
		id := c.Param("id")
		s.Audiences.Delete(id)
		if err := s.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(s.Cfg.Bucket.Audience)).Delete([]byte(id))
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		c.JSON(200, misc.StatusOK(""))
	}
}
