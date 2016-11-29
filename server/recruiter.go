package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

var ErrWait = errors.New("Waiting for the right moment")

func emailScraps(srv *Server) (int, error) {
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
	count := 0
	for _, sc := range scraps {
		if count >= 50 {
			// Only send 50 emails max per run
			break
		}

		if _, ok := signUps[strings.ToLower(sc.EmailAddress)]; ok {
			// This person has already signed up.. skip!
			continue
		}

		cmp := sc.GetMatchingCampaign(cmps)
		if cmp == nil {
			continue
		}

		if sent := sc.Email(cmp, srv.Cfg); !sent {
			continue
		}
		count += 1
		sc.SentEmails = append(sc.SentEmails, now)
	}

	if count > 0 {
		srv.Notify("Scraps Emailed", fmt.Sprintf("%d scraps emailed in last recruiter run!", count))
	}

	return count, saveScraps(srv, scraps)
}

func getAllScraps(s *Server) (scraps []*common.Scrap, err error) {
	if err = s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc common.Scrap
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

func saveScraps(s *Server, scs []*common.Scrap) error {
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