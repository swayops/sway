package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/influencer"
)

func newStatsUpdate(srv *Server) error {
	err := updateStats(srv, true)
	if err != nil {
		return err
	}

	// Check for approved deals every 6 hours
	ticker := time.NewTicker(10 * time.Second) //srv.Cfg.StatsUpdate * time.Hour)
	go func() {
		for range ticker.C {
			err = updateStats(srv, false)
			if err != nil {
				log.Println("Err with stats updater", err)
			}
		}
	}()
	return nil
}

func updateStats(s *Server, boot bool) error {
	// Traverses influencers and updates their stats
	if err := s.db.Update(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(s.Cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
			inf := influencer.Influencer{}
			if err := json.Unmarshal(v, &inf); err != nil {
				log.Println("error when unmarshalling influencer", string(v))
				return nil
			}

			if err := inf.UpdateAll(s.Cfg); err != nil {
				return err
			}

			if !boot {
				time.Sleep(s.Cfg.StatsInterval * time.Second)
			}

			// Update data for all completed deal posts
			for _, deal := range inf.CompletedDeals {
				if deal.Tweet != nil {
					if err := deal.Tweet.UpdateData(s.Cfg); err != nil {
						return err
					}
				} else if deal.Facebook != nil {
					if err := deal.Facebook.UpdateData(s.Cfg); err != nil {
						return err
					}
				} else if deal.Instagram != nil {
					if err := deal.Instagram.UpdateData(s.Cfg); err != nil {
						return err
					}
				} else if deal.YouTube != nil {
					if err := deal.YouTube.UpdateData(s.Cfg); err != nil {
						return err
					}
				}
			}

			// Save the influencer since we just updated it's data
			if err := saveInfluencer(tx, inf, s.Cfg); err != nil {
				return err
			}
			return
		})
		return nil
	}); err != nil {
		return err
	}
	return nil
}
