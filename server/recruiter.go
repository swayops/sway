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
	cmps := srv.Campaigns.GetStore()
	if len(cmps) == 0 {
		// We have no campaigns FOO
		return 0, nil
	}

	maxEmails := srv.Campaigns.GetAvailableDealCount() * 10

	// Influencers who have signed up
	signUps := srv.auth.Influencers.GetAllEmails()

	now := int32(time.Now().Unix())
	var count int32
	for _, sc := range srv.Scraps.GetStore() {
		if count >= maxEmails {
			// Only send max depending on how many deals available
			break
		}

		if _, ok := signUps[strings.ToLower(sc.EmailAddress)]; ok {
			// This person has already signed up.. skip!
			continue
		}

		cmp := sc.GetMatchingCampaign(cmps, srv.Audiences, srv.db, srv.Cfg)
		if cmp.Id == "" {
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
		store, err := budget.GetCampaignStoreFromDb(srv.db, srv.Cfg, cmp.Id, cmp.AdvertiserId)
		if err != nil || store == nil {
			continue
		}

		pendingSpend, _ := cmp.GetPendingDetails()
		availSpend := store.Spendable - pendingSpend

		// Only email them campaigns with more than $5
		if err != nil || store == nil || (availSpend < 20 && !cmp.IsProductBasedBudget()) {
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

		var dspFee, exchangeFee float64
		fees, ok := srv.Campaigns.GetAdvertiserFees(cmp.AdvertiserId)
		if ok {
			dspFee = fees.DSP
			exchangeFee = fees.Exchange
		} else {
			dspFee = -1
			exchangeFee = -1
		}

		maxYield := influencer.GetMaxYield(&cmp, sc.YTData, sc.FBData, sc.TWData, sc.InstaData)
		_, _, _, infPayout := budget.GetMargins(maxYield, dspFee, exchangeFee, -1)
		earnings := misc.TruncateFloat(infPayout, 2)
		if earnings <= 0 && !srv.Cfg.Sandbox {
			continue
		}

		if sent := sc.Email(&cmp, earnings, srv.Cfg); !sent {
			continue
		}
		count += 1
		sc.SentEmails = append(sc.SentEmails, now)
		if err := saveScrap(srv, sc); err != nil {
			srv.Alert("Error saving scrap", err)
		}

		// Update counts for notifications by campaigns
		if err := updateCampaignNotifications(srv, "sc-"+sc.Id, []string{cmp.Id}); err != nil {
			log.Println("Error when saving notifications for scrap", err, sc.Id, cmp.Id)
		}
	}

	return count, nil
}

func getAllScraps(s *Server) map[string]influencer.Scrap {
	scraps := make(map[string]influencer.Scrap)
	if err := s.db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Scrap)).ForEach(func(k, v []byte) (err error) {
			var sc influencer.Scrap
			if err := json.Unmarshal(v, &sc); err != nil {
				log.Println("error when unmarshalling scrap", string(v))
				return nil
			}
			scraps[sc.Id] = sc
			return
		})
		return nil
	}); err != nil {
		return scraps
	}
	return scraps
}

func saveScrap(s *Server, sc influencer.Scrap) error {
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

		s.Scraps.SetScrap(sc.Id, sc)

		return nil
	}); err != nil {
		s.Alert("Failed to save scraps!", err)
		return err
	}
	return nil
}

func saveScraps(s *Server, scs []influencer.Scrap) error {
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
			s.Scraps.SetScrap(sc.Id, sc)

		}
		return nil
	}); err != nil {
		s.Alert("Failed to save scraps!", err)
		return err
	}
	return nil
}
