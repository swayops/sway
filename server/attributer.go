package server

import (
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/platforms/facebook"
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

		if images := inf.GetImages(); len(images) > 0 {
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
	scraps, err := getAllScraps(srv)
	if err != nil {
		return updated, err
	}

	// Lets do batches of 500 so we don't max out API limits
	if len(scraps) > 500 {
		scraps = scraps[:500]
	}

	// Set keywords, geo, and followers for scraps!
	for _, sc := range scraps {
		if sc.Attributed {
			// This scrap already has attrs set
			continue
		}

		if sc.Instagram && sc.Name != "" {
			// This scrap is from instagram!
			insta, err := instagram.New(sc.Name, srv.Cfg)
			if err != nil {
				continue
			}

			sc.Images = insta.Images
			sc.Followers += int64(insta.Followers)
		} else if sc.YouTube && sc.Name != "" {
			// This scrap is from YT!
			yt, err := youtube.New(sc.Name, srv.Cfg)
			if err != nil {
				continue
			}

			sc.Images = yt.Images
			sc.Followers += int64(yt.Subscribers)
		} else if sc.Twitter && sc.Name != "" {
			tw, err := twitter.New(sc.Name, srv.Cfg)
			if err != nil {
				continue
			}
			sc.Geo = tw.LastLocation
			sc.Followers += int64(tw.Followers)
		} else if sc.Facebook && sc.Name != "" {
			fb, err := facebook.New(sc.Name, srv.Cfg)
			if err != nil {
				continue
			}
			sc.Followers += int64(fb.Followers)
		}

		keywords, err := imagga.GetKeywords(sc.Images, srv.Cfg.Sandbox)
		if err != nil {
			srv.Alert("Imagga error", err)
			continue
		}

		sc.Keywords = keywords
		sc.Attributed = true

		if err := saveScrap(srv, sc); err != nil {
			srv.Alert("Error saving scrap", err)
		}

		updated += 1

		if !force {
			time.Sleep(1 * time.Second)
		}
	}

	return updated, nil
}

func updateKeywords(s *Server, id string, keywords []string) error {
	inf, ok := s.auth.Influencers.Get(id)
	if !ok {
		return auth.ErrInvalidID
	}

	// Save the last email timestamp
	if err := s.db.Update(func(tx *bolt.Tx) error {
		inf.Keywords = append(inf.Keywords, keywords...)
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
