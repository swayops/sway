package influencer

import (
	"encoding/json"
	"log"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
)

func GetAllInfluencers(db *bolt.DB, cfg *config.Config) []*Influencer {
	influencers := make([]*Influencer, 0, 512)
	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Influencer)).ForEach(func(k, v []byte) (err error) {
			inf := Influencer{}
			if err := json.Unmarshal(v, &inf); err != nil {
				log.Println("Error when unmarshalling influencer", string(v))
				return nil
			}
			influencers = append(influencers, &inf)
			return
		})
		return nil
	}); err != nil {
		log.Println("Err when getting all influencers", err)
	}
	return influencers
}
