package budget

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/misc"
)

// Structure of Budget DB:
// {
// 	"01-2016": {
// 		"CID": {
// 			"budget": 20,
// 			"leftover": 10, // leftover from last month which was added
// 			"dspFee": 4, // amount dsp took
// 			"exchangeFee": 4, // amount exchange took
// 			"spendable": 10,
// 		}
// 	}
// }

var (
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
	ErrNotFound  = errors.New("CID not found!")
)

type Store struct {
	Budget    float64 `json:"budget,omitempty"`
	Pending   float64 `json:"pending,omitempty"`  // If the budget was lowered, this budget will be used next month
	Leftover  float64 `json:"leftover,omitempty"` // Left over budget from last month
	Spendable float64 `json:"spendable,omitempty"`
	Spent     float64 `json:"spent,omitempty"`

	DspFee      float64 `json:"dspFee,omitempty"`
	ExchangeFee float64 `json:"exchangeFee,omitempty"`
}

func CreateBudgetKey(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, leftover, pending, dspFee, exchangeFee float64, billing bool) (float64, error) {
	// Creates budget keys for NEW campaigns and campaigns on the FIRST OF THE MONTH!
	var spendable float64

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		key := getBudgetKey()
		b := tx.Bucket([]byte(cfg.BudgetBucket)).Get([]byte(key))

		var st map[string]*Store
		if len(b) == 0 {
			// First save of the month!
			st = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		monthlyBudget := cmp.Budget
		if !billing {
			// TODAY IS NOT BILLING DAY! (first of the month)
			// This function could be run could be mid-month.. (new campaign)
			// so we need to calculate what the given
			// (monthly) budget would be for the days left.
			now := time.Now().UTC()
			days := daysInMonth(now.Year(), now.Month())
			daysUntilEnd := days - now.Day() + 1

			monthlyBudget = (cmp.Budget / float64(days)) * float64(daysUntilEnd)
		} else {
			// TODAY IS BILLING DAY! (first of the month)
			// Is there a newBudget (pending) value (i.e. a lower budget)?
			if pending > 0 {
				// The budget was indeed lowered last month!
				// Use this as the new monthly budget
				monthlyBudget = pending
			}
		}

		// NOTE: This will automatically reset Pending too
		spendable = leftover + monthlyBudget
		store := &Store{
			Budget:      monthlyBudget,
			Leftover:    leftover,
			Spendable:   spendable,
			DspFee:      dspFee,
			ExchangeFee: exchangeFee,
		}

		st[cmp.Id] = store

		// Log the budget!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":     "insertion",
			"campaignId": cmp.Id,
			"store":      store,
		}); err != nil {
			log.Println("Failed to log budget insertion!", cmp.Id, store.Budget, store.Spendable, store.DspFee, store.ExchangeFee)
		}

		if b, err = json.Marshal(&st); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBucket, key, b); err != nil {
			return err
		}

		return nil
	}); err != nil {
		log.Println("Error when creating budget key", err)
		return 0, err
	}
	return spendable, nil
}

func AdjustBudget(db *bolt.DB, cfg *config.Config, cid string, newBudget, dspFee, exchangeFee float64) (float64, error) {
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return 0, err
	}

	store, ok := st[cid]
	if !ok {
		return 0, ErrNotFound
	}

	var tbaBudget float64

	oldBudget := store.Budget
	if newBudget > oldBudget {
		// If the budget has been INCREASED... increase spendable and fees
		diffBudget := newBudget - oldBudget

		// This function could be run could be mid-month..
		// so we need to calculate what the given
		// (monthly) budget would be for the days left.
		now := time.Now().UTC()
		days := daysInMonth(now.Year(), now.Month())
		daysUntilEnd := days - now.Day() + 1

		// The "to be added" value is based on the delta
		// between old and new budget and how many days are left
		// So if increase is $30 and the month has 10 days left..
		// $10 will be added to the budget
		tbaBudget = (diffBudget / float64(days)) * float64(daysUntilEnd)

		// Take out margins from spendable
		// NOTE: Leftover is not added to spendable because it already
		// should have been added last time billing ran!
		newStore := &Store{
			Budget:      oldBudget + tbaBudget,
			Leftover:    store.Leftover,
			Spendable:   store.Spendable + tbaBudget,
			Spent:       store.Spent,
			DspFee:      dspFee,
			ExchangeFee: exchangeFee,
		}

		st[cid] = newStore

		// Log the budget increase!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":      "increase",
			"campaignId":  cid,
			"store":       newStore,
			"addedBudget": tbaBudget,
		}); err != nil {
			log.Println("Failed to log budget decrease!", cid, tbaBudget, store.Budget, store.Spendable, store.DspFee, store.ExchangeFee, err)
		}

	} else if newBudget < oldBudget {
		// If the budget has DECREASED...
		// Save the budget in pending for when a transfer is made on the 1st
		newStore := &Store{
			Budget:      store.Budget,
			Leftover:    store.Leftover,
			Spendable:   store.Spendable,
			Spent:       store.Spent,
			DspFee:      store.DspFee,
			ExchangeFee: store.ExchangeFee,
			Pending:     newBudget,
		}

		st[cid] = newStore

		// Log the budget decrease!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":     "decrease",
			"campaignId": cid,
			"store":      newStore,
		}); err != nil {
			log.Println("Failed to log budget decrease!", cid, store.Pending, store.Budget, store.Spendable, store.DspFee, store.ExchangeFee, err)
		}
	}

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBucket, getBudgetKey(), b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when creating budget key", err)
		return 0, err
	}

	return tbaBudget, nil
}

func GetBudgetInfo(db *bolt.DB, cfg *config.Config, cid string, forceDate string) (*Store, error) {
	st, err := GetStore(db, cfg, forceDate)
	if err != nil {
		return nil, err
	}
	if store, ok := st[cid]; ok {
		return store, nil
	}
	return nil, ErrNotFound
}

func GetStore(db *bolt.DB, cfg *config.Config, forceDate string) (map[string]*Store, error) {
	// Gets budget store keyed off of Campaign ID for a given month
	var st map[string]*Store

	if err := db.View(func(tx *bolt.Tx) (err error) {
		key := forceDate
		if key == "" {
			key = getBudgetKey()
		}

		b := tx.Bucket([]byte(cfg.BudgetBucket)).Get([]byte(key))
		if err = json.Unmarshal(b, &st); err != nil {
			return ErrUnmarshal
		}
		return
	}); err != nil {
		return nil, err
	}

	return st, nil
}

type Metrics struct {
	Likes    int32 `json:"likes,omitempty"`
	Dislikes int32 `json:"dislikes,omitempty"`
	Comments int32 `json:"comments,omitempty"`
	Shares   int32 `json:"shares,omitempty"`
	Views    int32 `json:"views,omitempty"`
}

func AdjustStore(store *Store, deal *common.Deal) (*Store, float64, *Metrics) {
	// Add logging here eventually!

	m := &Metrics{}

	oldSpendable := store.Spendable
	if deal.Tweet != nil {
		// Considering retweets as shares and favorites as likes!
		m.Shares += int32(deal.Tweet.RetweetsDelta)
		m.Likes += int32(deal.Tweet.FavoritesDelta)

		store.deductSpendable(float64(deal.Tweet.RetweetsDelta) * TW_RETWEET)
		store.deductSpendable(float64(deal.Tweet.FavoritesDelta) * TW_FAVORITE)
	} else if deal.Facebook != nil {
		m.Likes += int32(deal.Facebook.LikesDelta)
		m.Shares += int32(deal.Facebook.SharesDelta)
		m.Comments += int32(deal.Facebook.CommentsDelta)

		store.deductSpendable(float64(deal.Facebook.LikesDelta) * FB_LIKE)
		store.deductSpendable(float64(deal.Facebook.SharesDelta) * FB_SHARE)
		store.deductSpendable(float64(deal.Facebook.CommentsDelta) * FB_COMMENT)
	} else if deal.Instagram != nil {
		m.Likes += int32(deal.Instagram.LikesDelta)
		m.Comments += int32(deal.Instagram.CommentsDelta)

		store.deductSpendable(float64(deal.Instagram.LikesDelta) * INSTA_LIKE)
		store.deductSpendable(float64(deal.Instagram.CommentsDelta) * INSTA_COMMENT)
	} else if deal.YouTube != nil {
		m.Views += int32(deal.YouTube.ViewsDelta)
		m.Likes += int32(deal.YouTube.LikesDelta)
		m.Comments += int32(deal.YouTube.CommentsDelta)

		store.deductSpendable(float64(deal.YouTube.ViewsDelta) * YT_VIEW)
		store.deductSpendable(float64(deal.YouTube.LikesDelta) * YT_LIKE)
		store.deductSpendable(float64(deal.YouTube.CommentsDelta) * YT_COMMENT)
	}

	spentDelta := oldSpendable - store.Spendable
	store.Spent += spentDelta

	// Clear out deltas since they've been used now!
	deal.ClearDeltas()

	return store, spentDelta, m
}

func SaveStore(db *bolt.DB, cfg *config.Config, store *Store, cid string) error {
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		key := getBudgetKey()
		b := tx.Bucket([]byte(cfg.BudgetBucket)).Get([]byte(key))

		var st map[string]*Store
		if len(b) == 0 {
			// First save of the month!
			st = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		st[cid] = store
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBucket, key, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when saving store", err)
		return err
	}
	return nil
}
