package influencer

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
)

func GetAllInfluencers(db *bolt.DB, cfg *config.Config) []*Influencer {
	var influencers []*Influencer
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

func GetInfluencerFromId(id string, db *bolt.DB, cfg *config.Config) (*Influencer, error) {
	var (
		v   []byte
		err error
		g   Influencer
	)

	if err := db.View(func(tx *bolt.Tx) error {
		v = tx.Bucket([]byte(cfg.Bucket.Influencer)).Get([]byte(id))
		return nil
	}); err != nil {
		return &g, err
	}

	if err = json.Unmarshal(v, &g); err != nil {
		return nil, err
	}

	return &g, nil
}

const dateFormat = "%d-%02d-%02d"

func getDate() string {
	return getDateFromTime(time.Now().UTC())
}

func getDateFromTime(t time.Time) string {
	return fmt.Sprintf(
		dateFormat,
		t.Year(),
		t.Month(),
		t.Day(),
	)
}

func degradeRep(val int32, rep float32) float32 {
	if val > 0 && val < 5 {
		rep = rep * 0.75
	} else if val >= 5 && val < 20 {
		rep = rep * 0.5
	} else if val >= 20 && val < 50 {
		rep = rep * 0.25
	} else if val >= 50 {
		rep = rep * 0.05
	}
	return rep
}
