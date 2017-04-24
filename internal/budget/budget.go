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
	"github.com/swayops/sway/platforms/swipe"
)

// Structure of Budget DB (Budget DB):
// {
// 	"01-2016": {
// 		"CID": {
// 			"budget": 20,
// 			"leftover": 10, // leftover from last month which was added
// 			"spendable": 10,
// 		}
// 	}
// }

// Structure of Budget DB (Balance DB):
// Value is balance available
// {
// 	"ADVERTISER ID": 234.234
// }

var (
	ErrUnmarshal = errors.New("Failed to unmarshal data!")
	ErrNotFound  = errors.New("CID not found!")
	ErrCC        = errors.New("Credit card not found!")
)

type Store struct {
	Spendable float64   `json:"spendable,omitempty"`
	Spent     float64   `json:"spent,omitempty"`   // Amount spent since last billing
	Charges   []*Charge `json:"charges,omitempty"` // Used for billing

	SpendHistory map[string]float64 `json:"spendHistory,omitempty"`
	NextBill     int64              `json:"nextBill,omitempty"` // When will this campaign be charged for again?
}

type Charge struct {
	Timestamp   int32   `json:"ts,omitempty"`
	Amount      float64 `json:"amount,omitempty"`
	FromBalance float64 `json:"fromBalance,omitempty"` // Amount used from balance
}

func (st *Store) AddCharge(amount, fromBalance float64) {
	charge := &Charge{
		Amount:      amount,
		Timestamp:   int32(time.Now().Unix()),
		FromBalance: fromBalance,
	}
	if st.Charges == nil {
		st.Charges = []*Charge{charge}
	} else {
		st.Charges = append(st.Charges, charge)
	}
}

func (st *Store) GetDelta() float64 {
	// Goes over spend values for the month and compares them with the charges.
	// Returns the dollar value for the amount that was SPENT but not CHARGED
	var charged float64
	for _, charge := range st.Charges {
		// Lets include the balance used AND the amount charged
		// Both are considered valid money sources before IO kicks in
		charged += charge.Amount + charge.FromBalance
	}

	delta := st.Spent - charged
	if delta < 0 {
		return 0
	}

	return delta
}

func (st *Store) IsClosed(cmp *common.Campaign) bool {
	// Is the store closed for business?
	return st == nil || (st.Spendable == 0 && !cmp.IsProductBasedBudget())
}

func (st *Store) Bill(cust string, pendingCharge float64, tx *bolt.Tx, cmp *common.Campaign, cfg *config.Config) error {
	var (
		balanceDeduction float64
		err              error
	)

	if pendingCharge == 0 {
		return nil
	}

	// If they have an available balance.. lets use it
	balance := GetBalance(cmp.AdvertiserId, tx, cfg)
	if balance > 0 {
		// We found some available funds! Lets deduct that from
		// the pending charge and only bill the delta
		if balance >= pendingCharge {
			// This means we can take the whole charge from the balance
			balanceDeduction = pendingCharge
			pendingCharge = 0
		} else {
			// Otherwise we need to deduct the FULL balance from the charge
			// and only charge the delta, and set balance to 0
			pendingCharge = pendingCharge - balance
			balanceDeduction = balance
		}
	}

	if pendingCharge > 0 {
		if err = swipe.Charge(cust, cmp.Name, cmp.Id, pendingCharge, balanceDeduction); err != nil {
			return err
		}
	}

	if balanceDeduction > 0 {
		// Lets deduct balance from the bucket if there is any!
		err := DeductBalance(cmp.AdvertiserId, balanceDeduction, tx, cfg)
		if err != nil {
			return err
		}
	}

	// Log the charge!
	if err := cfg.Loggers.Log("charge", map[string]interface{}{
		"campaignId":       cmp.Id,
		"charge":           pendingCharge,
		"balanceDeduction": balanceDeduction,
	}); err != nil {
		log.Println("Failed to log charge!", cmp.Id, pendingCharge, balanceDeduction)
	}

	st.AddCharge(pendingCharge, balanceDeduction)
	return nil
}

func RemoteBill(db *bolt.DB, cfg *config.Config, cid, advid string, isIO bool) error {
	// Creates budget keys for NEW campaigns
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(cmp.AdvertiserId))

		var st map[string]*Store
		if len(b) == 0 {
			// First save for this advertiser!
			st = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		oldStore, ok := st[cid]
		if !ok {
			return ErrNotFound
		}

		// Take today's date and add a month
		now := time.Now()
		nextBill := now.AddDate(0, 1, 0)

		if len(oldStore.SpendHistory) == 0 {
			oldStore.SpendHistory = make(map[string]float64)
		}

		oldStore.SpendHistory[GetSpendHistoryKey()] = oldStore.Spent

		store := &Store{
			Spendable: cmp.Budget + oldStore.Spendable,
			Spent:     0,
			Charges:   oldStore.Charges,

			NextBill:     nextBill.Unix(),
			SpendHistory: oldStore.SpendHistory,
		}

		// Charge the campaign for budget unless it's an IO campaign OR product based budget!
		if !isIO && !cmp.IsProductBasedBudget() {
			// CHARGE!
			if cust == "" {
				return ErrCC
			}

			if err := store.Bill(cust, cmp.Budget, tx, cmp, cfg); err != nil {
				return err
			}
		}

		st[cmp.Id] = store

		// Log the budget!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":     "refresh",
			"campaignId": cmp.Id,
			"store":      store,
			"io":         isIO,
		}); err != nil {
			log.Println("Failed to log budget insertion!", cmp.Id, cmp.Budget, store.Spendable)
			return err
		}

		if b, err = json.Marshal(&st); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func Create(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string) error {
	// Creates budget keys for NEW campaigns
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(cmp.AdvertiserId))

		var st map[string]*Store
		if len(b) == 0 {
			// First save for this advertiser!
			st = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		// Take today's date and add a month
		now := time.Now()
		nextBill := now.AddDate(0, 1, 0)

		store := &Store{
			Spendable: cmp.Budget,
			NextBill:  nextBill.Unix(),
		}

		// Charge the campaign for budget unless it's an IO campaign OR product based budget!
		if !isIO && !cmp.IsProductBasedBudget() {
			// CHARGE!
			if cust == "" {
				return ErrCC
			}

			if err := store.Bill(cust, cmp.Budget, tx, cmp, cfg); err != nil {
				return err
			}
		}

		st[cmp.Id] = store

		// Log the budget!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":     "insertion",
			"campaignId": cmp.Id,
			"store":      store,
			"io":         isIO,
		}); err != nil {
			log.Println("Failed to log budget insertion!", cmp.Id, cmp.Budget, store.Spendable)
			return err
		}

		if b, err = json.Marshal(&st); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func ReplenishSpendable(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string) error {
	st, err := GetAdvertiserStore(db, cfg, "")
	if err != nil {
		return err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return ErrNotFound
	}

	spendable := cmp.Budget - store.Spent
	if spendable < 0 {
		spendable = 0
	}

	newStore := &Store{
		Spent:     store.Spent,
		Charges:   store.Charges,
		Spendable: spendable,
	}

	if !isIO && !cmp.IsProductBasedBudget() {
		// CHARGE!
		if cust == "" {
			return ErrCC
		}

		if err := db.Update(func(tx *bolt.Tx) (err error) {
			if err := newStore.Bill(cust, spendable, tx, cmp, cfg); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	st[cmp.Id] = newStore

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving budget key", err)
		return err
	}

	return nil
}

func ClearSpendable(db *bolt.DB, cfg *config.Config, cmp *common.Campaign) (float64, error) {
	st, err := GetAdvertiserStore(db, cfg, cmp.AdvertiserId)
	if err != nil {
		return 0, err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return 0, ErrNotFound
	}

	// Save everything except the spendable! It's 0 now muahahaha
	newStore := &Store{
		Spent:   store.Spent,
		Charges: store.Charges,
	}

	st[cmp.Id] = newStore

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving budget key", err)
		return 0, err
	}

	return store.Spendable, nil
}

func GetCampaignStore(db *bolt.DB, cfg *config.Config, cid, advid string) (*Store, error) {
	st, err := GetAdvertiserStore(db, cfg, advid)
	if err != nil {
		return nil, err
	}
	if store, ok := st[cid]; ok {
		return store, nil
	}
	return nil, ErrNotFound
}

func GetStore(db *bolt.DB, cfg *config.Config) (map[string]*Store, error) {
	// Gets budget store keyed off of Campaign ID for a given month
	var store map[string]*Store
	if err := db.View(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cfg.Bucket.Budget)).ForEach(func(k, v []byte) (err error) {
			var st map[string]*Store
			if err := json.Unmarshal(v, &st); err != nil {
				log.Println("error when unmarshalling budget", string(v))
				return nil
			}

			for k, v := range st {
				store[k] = v
			}
			return
		})
		return nil
	}); err != nil {
		return store, err
	}

	return store, nil
}

func GetAdvertiserStore(db *bolt.DB, cfg *config.Config, advID string) (map[string]*Store, error) {
	// Gets budget store keyed off of Campaign ID for a given month
	var st map[string]*Store

	if err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(advID))
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
	var (
		shares, likes, comments, views int32
	)

	m := &Metrics{}

	oldSpendable := store.Spendable

	// We will use this to determine how many engagements have we already paid for
	// and pay engagement deltas using that!
	// This ensures that we pay for every engagement and makes payouts more robust.
	total := deal.TotalStats()

	if deal.Tweet != nil {
		// Considering retweets as shares and favorites as likes!

		// Subtracting all the engagements we have already recorded!
		shares = int32(deal.Tweet.Retweets) - total.Shares
		likes = int32(deal.Tweet.Favorites) - total.Likes

		m.Shares += shares
		m.Likes += likes

		store.deductSpendable(float64(shares) * TW_RETWEET)
		store.deductSpendable(float64(likes) * TW_FAVORITE)
	} else if deal.Facebook != nil {
		// Subtracting all the engagements we have already recorded!
		likes = int32(deal.Facebook.Likes) - total.Likes
		shares = int32(deal.Facebook.Shares) - total.Shares
		comments = int32(deal.Facebook.Comments) - total.Comments

		m.Likes += likes
		m.Shares += shares
		m.Comments += comments

		store.deductSpendable(float64(likes) * FB_LIKE)
		store.deductSpendable(float64(shares) * FB_SHARE)
		store.deductSpendable(float64(comments) * FB_COMMENT)
	} else if deal.Instagram != nil {
		likes = int32(deal.Instagram.Likes) - total.Likes
		comments = int32(deal.Instagram.Comments) - total.Comments

		m.Likes += likes
		m.Comments += comments

		store.deductSpendable(float64(likes) * INSTA_LIKE)
		store.deductSpendable(float64(comments) * INSTA_COMMENT)
	} else if deal.YouTube != nil {
		views = int32(deal.YouTube.Views) - total.Views
		likes = int32(deal.YouTube.Likes) - total.Likes
		comments = int32(deal.YouTube.Comments) - total.Comments

		m.Views += views
		m.Likes += likes
		m.Comments += comments

		store.deductSpendable(float64(views) * YT_VIEW)
		store.deductSpendable(float64(likes) * YT_LIKE)
		store.deductSpendable(float64(comments) * YT_COMMENT)
	}

	if len(total.PendingClicks) > 0 {
		// Lets pay for all the pending clicks!
		store.deductSpendable(float64(len(total.PendingClicks)) * CLICK)
	}

	spentDelta := oldSpendable - store.Spendable
	store.Spent += spentDelta

	return store, spentDelta, m
}

func SaveStore(db *bolt.DB, cfg *config.Config, store *Store, cmp *common.Campaign) error {
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(cmp.AdvertiserId))

		var st map[string]*Store
		if len(b) == 0 {
			// First save of the month!
			st = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &st); err != nil {
				return ErrUnmarshal
			}
		}

		st[cmp.Id] = store
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when saving store", err)
		return err
	}
	return nil
}
