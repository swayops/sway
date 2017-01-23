package server

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/genderize"
	"github.com/swayops/sway/platforms/imagga"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

func attributer(srv *Server, force bool) (int64, error) {
	// Assigns missing attributes to scraps and influencers
	var updated int64
	// Iterate over all influencers and add keywords for them (if they don't have any)
	for _, inf := range srv.auth.Influencers.GetAll() {
		if len(inf.Keywords) > 0 {
			// Only append keywords if they don't have any
			continue
		}

		if images := inf.GetImages(srv.Cfg); len(images) > 0 {
			keywords, err := imagga.GetKeywords(images, srv.Cfg.Sandbox)
			if err != nil {
				srv.Alert("Imagga error", err)
				continue
			}

			if len(keywords) > 0 {
				if err := updateKeywords(srv, inf.Id, keywords); err != nil {
					srv.Alert("Error saving keywords!", err)
				}
				updated += 1
			}
		}
	}

	// Iterate over all scraps and add keywords for them (if they don't have any)
	scraps, err := getAllNewScraps(srv)
	if err != nil {
		return updated, err
	}

	// Lets do batches of 500 so we don't max out API limits
	var scrapsTouched int64
	// Set keywords, geo, gender, and followers for scraps!
	for _, sc := range scraps {
		if sc.Attributed || sc.Attempts > 3 {
			// This scrap already has attrs set
			continue
		}

		// Lets save the attempt right off the bat
		sc.Attempts += 1
		if err := saveScrap(srv, sc); err != nil {
			srv.Alert("Error saving scrap", err)
		}

		var (
			images []string
		)

		if sc.Instagram && sc.Name != "" {
			// We only extract location, gender, name, and followers from
			// insta!

			// This scrap is from instagram!
			insta, err := instagram.New(sc.Name, srv.Cfg)
			if err != nil {
				if !force {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			if insta.Followers > 0 {
				sc.Followers = int64(insta.Followers)
			}

			images = insta.Images

			if insta.LastLocation != nil {
				sc.Geo = insta.LastLocation
			}

			if insta.FullName != "" && sc.FullName == "" {
				// If we found a name and their gender has NOT been set
				// yet..
				sc.FullName = insta.FullName
			}
		} else if sc.YouTube && sc.Name != "" {
			// This scrap is from YT!
			yt, err := youtube.New(sc.Name, srv.Cfg)
			if err != nil {
				if !force {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			if yt.Subscribers > 0 {
				sc.Followers = int64(yt.Subscribers)
			}

			images = yt.Images
		} else if sc.Twitter && sc.Name != "" {
			tw, err := twitter.New(sc.Name, srv.Cfg)
			if err != nil {
				if !force {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			if tw.Followers > 0 {
				sc.Followers = int64(tw.Followers)
			}

			if tw.LastLocation != nil {
				sc.Geo = tw.LastLocation
			}

			if tw.FullName != "" && sc.FullName == "" {
				sc.FullName = tw.FullName
			}
		} else if sc.Facebook && sc.Name != "" {
			fb, err := facebook.New(sc.Name, srv.Cfg)
			if err != nil {
				if !force {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			if fb.Followers > 0 {
				sc.Followers = int64(fb.Followers)
			}
		}

		// Set keywords based on images!
		if len(sc.Keywords) == 0 {
			// Only hit imagga if keywords have not already been set
			keywords, err := imagga.GetKeywords(images, srv.Cfg.Sandbox)
			if err != nil {
				srv.Alert("Imagga error", err)
				continue
			}
			sc.Keywords = keywords
		}

		// Set categories based on keywords!
		if len(images) > 0 && len(sc.Keywords) > 0 {
			// If keywords are now set.. lets map them to categories
			sc.Categories = common.KwToCategories(sc.Keywords)
		}

		// Set gender based on name!
		if !sc.Male && !sc.Female && sc.FullName != "" {
			// If gender has not been set and we have a full name..
			firstName := genderize.GetFirstName(sc.FullName)
			sc.Male, sc.Female = genderize.GetGender(firstName)
		}

		sc.Attributed = true

		if err := saveScrap(srv, sc); err != nil {
			srv.Alert("Error saving scrap", err)
		}

		updated += 1
		scrapsTouched += 1

		// Do batches of 2500
		if scrapsTouched >= 2500 {
			break
		}

		if !force {
			time.Sleep(1 * time.Second)
		}
	}

	if updated > 0 {
		srv.Notify("Attribution ran!", "Attributed users: "+strconv.Itoa(int(updated)))
	}

	return updated, nil
}

func updateKeywords(s *Server, id string, keywords []string) error {
	inf, ok := s.auth.Influencers.Get(id)
	if !ok {
		return auth.ErrInvalidID
	}

	if err := s.db.Update(func(tx *bolt.Tx) error {
		for _, kw := range keywords {
			if !common.IsInList(inf.Keywords, kw) {
				inf.Keywords = append(inf.Keywords, kw)
			}
		}

		// Save the influencer since we just updated it's keywords
		if err := saveInfluencer(s, tx, inf); err != nil {
			log.Println("Errored saving influencer", err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func getAllKeywords(srv *Server) (keywords []string) {
	set := make(map[string]struct{})
	for _, inf := range srv.auth.Influencers.GetAll() {
		for _, kw := range inf.Keywords {
			set[kw] = struct{}{}
		}
	}

	scraps, _ := getAllScraps(srv)
	for _, sc := range scraps {
		for _, kw := range sc.Keywords {
			set[kw] = struct{}{}
		}
	}

	for kw, _ := range set {
		keywords = append(keywords, kw)
	}

	return
}

func assignGeo(srv *Server) (err error) {
	// Iterate over all scraps and add geo for insta users!
	scraps, err := getAllScraps(srv)
	if err != nil {
		return err
	}

	for _, sc := range scraps {
		if sc.Geo != nil || sc.Attempts > 3 {
			continue
		}

		if sc.Instagram && sc.Name != "" {
			insta, err := instagram.New(sc.Name, srv.Cfg)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if insta.LastLocation != nil {
				sc.Geo = insta.LastLocation
			}

			if err := saveScrap(srv, sc); err != nil {
				srv.Alert("Error saving scrap", err)
			}
		}

		time.Sleep(1 * time.Second)
	}
	return nil
}

func getAllNewScraps(s *Server) (scraps []*influencer.Scrap, err error) {
	// Gets all scraps that have not been attribuetd
	if err = s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc influencer.Scrap
			if err := json.Unmarshal(v, &sc); err != nil {
				log.Println("error when unmarshalling scrap", string(v))
				return nil
			}
			if !sc.Attributed {
				scraps = append(scraps, &sc)
			}
			return
		})
		return nil
	}); err != nil {
		return
	}
	return
}
