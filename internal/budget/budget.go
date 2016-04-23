package budget

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/reporting"
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
// 			"agencyFee": 4, // amount talent agency took
// 			"spendable": 10,
// 			"influencers": {
// 				"JennaMarbles": {
// 					"payout": 3.4,
// 				}
// 			}
// 		}
// 	}
// }

var (
	TalentAgencyFee = float32(0.125) // 12.5%
	ErrUnmarshal    = errors.New("Failed to unmarshal data!")
	ErrNotFound     = errors.New("CID not found!")
)

type Store struct {
	Budget    float32 `json:"budget,omitempty"`
	Pending   float32 `json:"pending,omitempty"`  // If the budget was lowered, this budget will be used next month
	Leftover  float32 `json:"leftover,omitempty"` // Left over budget from last month
	Spendable float32 `json:"spendable,omitempty"`
	Spent     float32 `json:"spent,omitempty"`

	DspFee      float32                    `json:"dspFee,omitempty"`
	ExchangeFee float32                    `json:"exchangeFee,omitempty"`
	Influencers map[string]*InfluencerData `json:"influencers,omitempty"`
}

type InfluencerData struct {
	Payout float32 `json:"payout,omitempty"`
}

func CreateBudgetKey(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, leftover, pending, dspFee, exchangeFee float32, billing bool) error {
	// Creates budget keys for NEW campaigns and campaigns on the FIRST OF THE MONTH!
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

			monthlyBudget = (cmp.Budget / float32(days)) * float32(daysUntilEnd)
		} else {
			// TODAY IS BILLING DAY! (first of the month)
			// Is there a newBudget value (i.e. a lower budget)?
			if pending > 0 {
				// The budget was indeed lowered last month!
				// Use this as the new daily budget
				monthlyBudget = pending
			}
		}

		dspCut := monthlyBudget * dspFee
		exchangeCut := monthlyBudget * exchangeFee

		// Take out margins from spendable
		// NOTE: This will automatically reset Pending too
		// Also.. no need for an influencers struct because
		// this is either a brand new campaign or its billing
		// day
		st[cmp.Id] = &Store{
			Budget:      monthlyBudget,
			Leftover:    leftover,
			Spendable:   leftover + monthlyBudget - (dspCut + exchangeCut),
			DspFee:      dspCut,
			ExchangeFee: exchangeCut,
		}

		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBucket, key, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when creating budget key", err)
		return err
	}
	return nil
}

func AdjustBudget(db *bolt.DB, cfg *config.Config, cid string, newBudget, dspFee, exchangeFee float32) error {
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return err
	}

	store, ok := st[cid]
	if !ok {
		return ErrNotFound
	}

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

		tbaBudget := (diffBudget / float32(days)) * float32(daysUntilEnd)

		tbaDspFee := tbaBudget * dspFee
		tbaExchangeFee := tbaBudget * exchangeFee

		// Take out margins from spendable
		// NOTE: Leftover is not added to spendable because it already
		// should have been added last time billing ran!
		st[cid] = &Store{
			Budget:      oldBudget + tbaBudget,
			Leftover:    store.Leftover,
			Spendable:   store.Spendable + tbaBudget - (tbaDspFee + tbaExchangeFee),
			Spent:       store.Spent,
			DspFee:      store.DspFee + tbaDspFee,
			ExchangeFee: store.ExchangeFee + tbaExchangeFee,
			Influencers: store.Influencers,
		}
	} else if newBudget < oldBudget {
		// If the budget has DECREASED...
		// Save the budget in pending for when a transfer is made on the 1st
		st[cid] = &Store{
			Budget:      store.Budget,
			Leftover:    store.Leftover,
			Spendable:   store.Spendable,
			Spent:       store.Spent,
			DspFee:      store.DspFee,
			ExchangeFee: store.ExchangeFee,
			Pending:     newBudget,
			Influencers: store.Influencers,
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
		return err
	}

	return nil
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
		log.Println("Error when getting store", err)
		return nil, err
	}

	return st, nil
}

func AdjustStore(store *Store, deal *common.Deal, stats *reporting.Stats) (*Store, *reporting.Stats) {
	if store.Influencers == nil || len(store.Influencers) == 0 {
		store.Influencers = make(map[string]*InfluencerData)
	}

	infData, ok := store.Influencers[deal.InfluencerId]
	if !ok {
		// Saving the pointer once
		infData = &InfluencerData{}
		store.Influencers[deal.InfluencerId] = infData
	}

	// Add logging here eventually!

	oldSpendable := store.Spendable
	if deal.Tweet != nil {
		// Considering retweets as shares and favorites as likes!
		stats.Shares += int32(deal.Tweet.RetweetsDelta)
		stats.Likes += int32(deal.Tweet.FavoritesDelta)
		stats.Published = int32(deal.Tweet.CreatedAt.Unix())

		store.deductSpendable(float32(deal.Tweet.RetweetsDelta) * TW_RETWEET)
		store.deductSpendable(float32(deal.Tweet.FavoritesDelta) * TW_FAVORITE)
	} else if deal.Facebook != nil {
		stats.Likes += int32(deal.Facebook.LikesDelta)
		stats.Shares += int32(deal.Facebook.SharesDelta)
		stats.Comments += int32(deal.Facebook.CommentsDelta)
		stats.Published = int32(deal.Facebook.Published.Unix())

		store.deductSpendable(float32(deal.Facebook.LikesDelta) * FB_LIKE)
		store.deductSpendable(float32(deal.Facebook.SharesDelta) * FB_SHARE)
		store.deductSpendable(float32(deal.Facebook.CommentsDelta) * FB_COMMENT)
	} else if deal.Instagram != nil {
		stats.Likes += int32(deal.Instagram.LikesDelta)
		stats.Comments += int32(deal.Instagram.CommentsDelta)
		stats.Published = deal.Instagram.Published

		store.deductSpendable(float32(deal.Instagram.LikesDelta) * INSTA_LIKE)
		store.deductSpendable(float32(deal.Instagram.CommentsDelta) * INSTA_COMMENT)
	} else if deal.YouTube != nil {
		stats.Views += int32(deal.YouTube.ViewsDelta)
		stats.Likes += int32(deal.YouTube.LikesDelta)
		stats.Comments += int32(deal.YouTube.CommentsDelta)
		stats.Published = deal.YouTube.Published

		store.deductSpendable(float32(deal.YouTube.ViewsDelta) * YT_VIEW)
		store.deductSpendable(float32(deal.YouTube.LikesDelta) * YT_LIKE)
		store.deductSpendable(float32(deal.YouTube.CommentsDelta) * YT_COMMENT)
	}

	tmpSpent := oldSpendable - store.Spendable

	store.Spent += tmpSpent
	infData.Payout += tmpSpent

	stats.Payout += tmpSpent

	return store, stats
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
