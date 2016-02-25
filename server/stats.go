package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/influencer"
)

func newStatsUpdate(srv *Server) error {
	// Update social media profiles every X hours
	ticker := time.NewTicker(srv.Cfg.StatsUpdate * time.Hour)
	go func() {
		if err := updateStats(srv); err != nil {
			log.Println("Err with stats updater", err)
		}
		for range ticker.C {
			if err := updateStats(srv); err != nil {
				log.Println("Err with stats updater", err)
			}
		}
	}()
	return nil
}

func updateStats(s *Server) error {
	// Traverses all influencers and updates their social media stats
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

			// Inserting a request interval so we don't hit our API
			// limits with platforms!
			time.Sleep(s.Cfg.StatsInterval * time.Second)

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

			time.Sleep(s.Cfg.StatsInterval * time.Second)

			// Save the influencer since we just updated it's social media data
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
