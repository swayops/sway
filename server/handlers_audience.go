package server

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

func adminAudience(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud, err := createAudienceHelper(s, c, false, false)
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(aud.Id))
	}
}

func agencyAudience(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud, err := createAudienceHelper(s, c, true, false)
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(aud.Id))
	}
}

func advertiserAudience(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud, err := createAudienceHelper(s, c, false, true)
		if err != nil {
			misc.WriteJSON(c, 400, misc.StatusErr(err.Error()))
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(aud.Id))
	}
}

func createAudienceHelper(s *Server, c *gin.Context, agency, advertiser bool) (aud common.Audience, err error) {
	// Ingests a list of emails, audience name and saves (or overwrites existing ID)
	defer c.Request.Body.Close()
	if err = json.NewDecoder(c.Request.Body).Decode(&aud); err != nil {
		err = errors.New("Error unmarshalling request body")
		return
	}

	if aud.Token != "" {
		// If a token is passed in, that implies UI would like to dump
		// the saved users into the audience (all of them!)
		if infs, _, _, ok := s.Forecasts.Get(aud.Token, 0, 100000, false); ok {
			aud.Members = convertToMap(infs)
		}
		aud.Token = ""
	}

	if len(aud.Members) == 0 {
		err = errors.New("Please provide a valid audience list")
		return
	}
	aud.Members = common.TrimEmails(aud.Members)

	if aud.Name == "" {
		err = errors.New("Please provide a valid audience name")
		return
	}

	idPrefix := c.Param("id")

	// If an ID is not passed in we assume a new audience is being made
	if aud.Id == "" {
		if err = s.db.Update(func(tx *bolt.Tx) (err error) { // have to get an id early for saveImage
			aud.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Audience)
			if idPrefix != "" {
				if agency {
					aud.Id = "agency:" + idPrefix + ":" + aud.Id
				} else if advertiser {
					aud.Id = "advertiser:" + idPrefix + ":" + aud.Id
				}
			}
			return
		}); err != nil {
			return
		}
	}

	if aud.ImageData != "" {
		if !strings.HasPrefix(aud.ImageData, "data:image/") {
			return aud, errors.New("Please provide a valid audience image")
		}

		// NOTE FOR Ahmed: Change min size and height as per ur liking
		filename, err := saveImageToDisk(filepath.Join(s.Cfg.ImagesDir, s.Cfg.Bucket.Audience, aud.Id), aud.ImageData, aud.Id, "", 750, 389)
		if err != nil {
			return aud, err
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
		return
	}

	return aud, nil
}

func getAudiences(s *Server) gin.HandlerFunc {
	// Optional "ID" param to filter to one audience, otherwise it returns
	// ALL admin audiences
	return func(c *gin.Context) {
		misc.WriteJSON(c, 200, s.Audiences.GetAdminStore(c.Param("id")))
	}
}

func getAudience(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud, _ := s.Audiences.Get(c.Param("audID"))
		misc.WriteJSON(c, 200, aud)
	}
}

func delAudience(s *Server) gin.HandlerFunc {
	// Delete given admin audience with id "id"
	return func(c *gin.Context) {
		id := c.Param("id")
		s.Audiences.Delete(id)
		if err := s.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(s.Cfg.Bucket.Audience)).Delete([]byte(id))
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

func getUserAudiences(s *Server) gin.HandlerFunc {
	// Get relevant audiences for the given user
	return func(c *gin.Context) {
		user := s.auth.GetUser(c.Param("id"))
		if user == nil {
			misc.WriteJSON(c, 400, misc.StatusErr("Please provide a valid user ID"))
			return
		}

		// Lets initialize with all the admin level audiences
		baseAudience := s.Audiences.GetAdminStore("")
		if user.Advertiser != nil {
			// This person is an advertiser! Lets get their advertiser level audiences
			// and add it
			for k, v := range s.Audiences.GetStoreByFilter(user.Advertiser.ID, false) {
				baseAudience[k] = v
			}

			// Lets add their agency level audiences too
			for k, v := range s.Audiences.GetStoreByFilter(user.Advertiser.AgencyID, false) {
				baseAudience[k] = v
			}
		} else if user.AdAgency != nil {
			// This person is an agency.. just add their agency audiences
			for k, v := range s.Audiences.GetStoreByFilter(user.Advertiser.AgencyID, false) {
				baseAudience[k] = v
			}
		}
		misc.WriteJSON(c, 200, baseAudience)
	}
}

func delAdvertiserAudience(s *Server) gin.HandlerFunc {
	// Delete audience for the given advertiser
	return func(c *gin.Context) {
		id := c.Param("audID")
		s.Audiences.Delete(id)
		if err := s.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(s.Cfg.Bucket.Audience)).Delete([]byte(id))
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}

func getAudiencesByAgency(s *Server) gin.HandlerFunc {
	// Get audiences for given agency id
	return func(c *gin.Context) {
		misc.WriteJSON(c, 200, s.Audiences.GetStoreByFilter(c.Param("id"), true))
	}
}

func delAgencyAudience(s *Server) gin.HandlerFunc {
	// Delete audience for the given agency id
	return func(c *gin.Context) {
		id := c.Param("audID")
		s.Audiences.Delete(id)
		if err := s.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(s.Cfg.Bucket.Audience)).Delete([]byte(id))
		}); err != nil {
			misc.AbortWithErr(c, 500, err)
			return
		}

		misc.WriteJSON(c, 200, misc.StatusOK(""))
	}
}
