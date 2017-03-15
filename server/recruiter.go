package server

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/budget"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/misc"
)

var ErrWait = errors.New("Waiting for the right moment")

func emailScraps(srv *Server) (int32, error) {
	// This should trigger X amount of emails daily
	// with varying templates and should ONLY begin
	// emailing once a campaign is live
	scraps, err := getAllScraps(srv)
	if err != nil {
		return 0, err
	}

	cmps := srv.Campaigns.GetStore()
	if len(cmps) == 0 {
		// We have no campaigns FOO
		return 0, ErrWait
	}

	// Influencers who have signed up
	signUps := srv.auth.Influencers.GetAllEmails()

	now := int32(time.Now().Unix())
	var count int32
	for _, sc := range scraps {
		if count >= 5 {
			// Only send 5 emails max per run
			break
		}

		if _, ok := signUps[strings.ToLower(sc.EmailAddress)]; ok {
			// This person has already signed up.. skip!
			continue
		}

		cmp := sc.GetMatchingCampaign(cmps, srv.budgetDb, srv.Cfg)
		if cmp == nil {
			continue
		}

		// SANITY CHECKS!
		if _, alive := srv.Campaigns.Get(cmp.Id); !alive {
			continue
		}

		if len(cmp.Whitelist) > 0 {
			continue
		}

		var spendable float64
		store, err := budget.GetBudgetInfo(srv.budgetDb, srv.Cfg, cmp.Id, "")
		// Only email them campaigns with more than $5
		if err != nil || store == nil || (store.Spendable < 5 && !cmp.IsProductBasedBudget()) {
			if srv.Cfg.Sandbox {
				// Don't throw out for sandbox requests.. because earlier tests (before TestScraps)
				// empty out the spendable.. so campaigns are always thrown out!
				spendable = 69.999
			} else {
				continue
			}
		} else {
			spendable = store.Spendable
		}

		if spendable == 0 && !cmp.IsProductBasedBudget() {
			continue
		}

		if sent := sc.Email(cmp, spendable, srv.Cfg); !sent {
			continue
		}
		count += 1
		sc.SentEmails = append(sc.SentEmails, now)
		if err := saveScrap(srv, sc); err != nil {
			srv.Alert("Error saving scrap", err)
		}
	}

	return count, nil
}

func getAllScraps(s *Server) (scraps []*influencer.Scrap, err error) {
	if err = s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc influencer.Scrap
			if err := json.Unmarshal(v, &sc); err != nil {
				log.Println("error when unmarshalling scrap", string(v))
				return nil
			}
			scraps = append(scraps, &sc)
			return
		})
		return nil
	}); err != nil {
		return
	}
	return
}

func saveScrap(s *Server, sc *influencer.Scrap) error {
	if err := s.db.Update(func(tx *bolt.Tx) (err error) {
		if sc.Id == "" {
			if sc.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Scrap); err != nil {
				return err
			}
		}

		var (
			b []byte
		)

		// Sanitize the email
		sc.EmailAddress = misc.TrimEmail(sc.EmailAddress)

		if b, err = json.Marshal(sc); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Scrap, sc.Id, b); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Alert("Failed to save scraps!", err)
		return err
	}
	return nil
}

func saveScraps(s *Server, scs []*influencer.Scrap) error {
	if err := s.db.Update(func(tx *bolt.Tx) (err error) {
		for _, sc := range scs {
			if sc.Id == "" {
				if sc.Id, err = misc.GetNextIndex(tx, s.Cfg.Bucket.Scrap); err != nil {
					continue
				}
			}

			var (
				b   []byte
				err error
			)

			// Sanitize the email
			sc.EmailAddress = misc.TrimEmail(sc.EmailAddress)

			if b, err = json.Marshal(sc); err != nil {
				continue
			}

			if err = misc.PutBucketBytes(tx, s.Cfg.Bucket.Scrap, sc.Id, b); err != nil {
				continue
			}
		}
		return nil
	}); err != nil {
		s.Alert("Failed to save scraps!", err)
		return err
	}
	return nil
}

// Used for retrieving keywords when a scrap signs up
func getScrapKeywords(s *Server, email, id string) (keywords []string) {
	if err := s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc influencer.Scrap
			if err := json.Unmarshal(v, &sc); err != nil {
				log.Println("error when unmarshalling scrap", string(v))
				return nil
			}

			if strings.EqualFold(sc.EmailAddress, email) && strings.EqualFold(sc.Name, id) && len(sc.Keywords) > 0 {
				keywords = sc.Keywords
				if s.Cfg.Sandbox {
					// Lets prepend everything with a prefix so we know where the kw is coming
					// from in tests
					keywords = prepend(sc.Keywords)
				}
				return errors.New("Done!")
			}

			return nil
		})
		return nil
	}); err != nil {
		return keywords
	}
	return keywords
}

func prepend(keywords []string) []string {
	out := []string{}
	for _, kw := range keywords {
		out = append(out, "old-"+kw)
	}
	return out
}
