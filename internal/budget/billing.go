package budget

import (
	"log"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

// Billing key will store the month billing last ran for
var (
	billingKey = "lastBill"
	oneWeek    = 604800 // One week in seconds
)

func ShouldBill(db *bolt.DB, cfg *config.Config) bool {
	// If it's the first day of the month and billing hasn't ran in a week.. bill
	if isFirstDay() && (int(time.Now().Unix())-oneWeek) > getLastBill(db, cfg) {
		return true
	}
	return false
}

func getLastBill(db *bolt.DB, cfg *config.Config) int {
	var st int
	if err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.BudgetBucket)).Get([]byte(billingKey))
		if len(b) == 0 {
			return nil
		}
		st, _ = strconv.Atoi(string(b))
		return nil
	}); err != nil {
		log.Println("Error when getting last bill", err)
		return st
	}
	return st
}

func UpdateLastBill(db *bolt.DB, cfg *config.Config) error {
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		val := strconv.Itoa(int(time.Now().Unix()))
		if err = misc.PutBucketBytes(tx, cfg.BudgetBucket, billingKey, []byte(val)); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving last bill", err)
		return err
	}
	return nil
}
