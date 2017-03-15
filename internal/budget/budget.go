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
	Budget    float64   `json:"budget,omitempty"`
	Pending   float64   `json:"pending,omitempty"`  // If the budget was lowered, this budget will be used next month
	Leftover  float64   `json:"leftover,omitempty"` // Left over budget from last month
	Spendable float64   `json:"spendable,omitempty"`
	Spent     float64   `json:"spent,omitempty"`
	Charges   []*Charge `json:"charges,omitempty"` // Used for billing
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

func (st *Store) IsClosed(cmp common.Campaign) bool {
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

func CreateBudgetKey(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, leftover, pending float64, billing, isIO bool, cust string) (float64, error) {
	// Creates budget keys for NEW campaigns and campaigns on the FIRST OF THE MONTH!
	var spendable float64
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		key := getBudgetKey()
		b := tx.Bucket([]byte(cfg.BudgetBuckets.Budget)).Get([]byte(key))

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
			monthlyBudget = GetProratedBudget(cmp.Budget)
			if cfg.Sandbox && !cmp.IsProductBasedBudget() {
				monthlyBudget = cmp.Budget
			}
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
			Budget:    cmp.Budget,
			Leftover:  leftover,
			Spendable: spendable,
		}

		// Charge the campaign for budget unless it's an IO campaign OR product based budget!
		if !isIO && !cmp.IsProductBasedBudget() {
			// CHARGE!
			if cust == "" {
				return ErrCC
			}

			if err := store.Bill(cust, monthlyBudget, tx, cmp, cfg); err != nil {
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
			log.Println("Failed to log budget insertion!", cmp.Id, store.Budget, store.Spendable)
			return err
		}

		if b, err = json.Marshal(&st); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, key, b); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return 0, err
	}
	return spendable, nil
}

func AdjustBudget(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, newBudget float64, isIO bool, cust string) (float64, error) {
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return 0, err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return 0, ErrNotFound
	}

	var tbaBudget float64

	oldBudget := store.Budget
	if newBudget > oldBudget {
		// If the budget has been INCREASED... increase spendable and fees
		diffBudget := newBudget - oldBudget

		// The "to be added" value is based on the delta
		// between old and new budget and how many days are left
		// So if increase is $30 and the month has 10 days left..
		// $10 will be added to the budget
		tbaBudget = GetProratedBudget(diffBudget)

		// Take out margins from spendable
		// NOTE: Leftover is not added to spendable because it already
		// should have been added last time billing ran!
		newStore := &Store{
			Budget:    oldBudget + tbaBudget,
			Leftover:  store.Leftover,
			Spendable: store.Spendable + tbaBudget,
			Spent:     store.Spent,
			Charges:   store.Charges,
		}

		if !isIO {
			// CHARGE!
			if cust == "" {
				return 0, ErrCC
			}

			if err := db.Update(func(tx *bolt.Tx) (err error) {
				if err := newStore.Bill(cust, tbaBudget, tx, cmp, cfg); err != nil {
					return err
				}
				return nil
			}); err != nil {
				return 0, err
			}
		}

		st[cmp.Id] = newStore

		// Log the budget increase!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":      "increase",
			"campaignId":  cmp.Id,
			"store":       newStore,
			"addedBudget": tbaBudget,
		}); err != nil {
			log.Println("Failed to log budget decrease!", cmp.Id, tbaBudget, store.Budget, store.Spendable, err)
			return 0, err
		}

	} else if newBudget < oldBudget {
		// If the budget has DECREASED...
		// Save the budget in pending for when a transfer is made on the 1st
		newStore := &Store{
			Budget:    store.Budget,
			Leftover:  store.Leftover,
			Spendable: store.Spendable,
			Spent:     store.Spent,
			Charges:   store.Charges,
			Pending:   newBudget,
		}

		st[cmp.Id] = newStore

		// Log the budget decrease!
		if err := cfg.Loggers.Log("stats", map[string]interface{}{
			"action":     "decrease",
			"campaignId": cmp.Id,
			"store":      newStore,
		}); err != nil {
			log.Println("Failed to log budget decrease!", cmp.Id, store.Pending, store.Budget, store.Spendable, err)
			return 0, err
		}
	}

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, getBudgetKey(), b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when creating budget key", err)
		return 0, err
	}

	return tbaBudget, nil
}

func ReplenishSpendable(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string) error {
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return ErrNotFound
	}

	spendable := GetProratedBudget(cmp.Budget) - store.Spent
	if spendable < 0 {
		spendable = 0
	}

	newStore := &Store{
		Budget:    store.Budget,
		Leftover:  store.Leftover,
		Spent:     store.Spent,
		Charges:   store.Charges,
		Pending:   store.Pending,
		Spendable: spendable,
	}

	if !isIO {
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

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, getBudgetKey(), b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving budget key", err)
		return err
	}

	return nil
}

func Credit(db *bolt.DB, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string, credit float64) error {
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return ErrNotFound
	}

	spendable := credit + store.Spendable
	if spendable < 0 {
		spendable = 0
	}

	newStore := &Store{
		Budget:    store.Budget,
		Leftover:  store.Leftover,
		Spent:     store.Spent,
		Charges:   store.Charges,
		Pending:   store.Pending,
		Spendable: spendable,
	}

	if !isIO {
		// CHARGE!
		if cust == "" {
			return ErrCC
		}

		if err := db.Update(func(tx *bolt.Tx) (err error) {
			if err := newStore.Bill(cust, cmp.Budget, tx, cmp, cfg); err != nil {
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

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, getBudgetKey(), b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving budget key", err)
		return err
	}

	return nil
}

func TransferSpendable(db *bolt.DB, cfg *config.Config, cmp *common.Campaign) error {
	// Transfers spendable from last month to this month

	oldStore, err := GetStore(db, cfg, GetLastMonthBudgetKey())
	if err != nil {
		return err
	}

	cmpStore, ok := oldStore[cmp.Id]
	if !ok {
		return ErrNotFound
	}

	if cmpStore.Spendable == 0 {
		return ErrNotFound
	}

	oldSpendable := cmpStore.Spendable

	// Lets give this spendable for thsi month!
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		key := getBudgetKey()
		b := tx.Bucket([]byte(cfg.BudgetBuckets.Budget)).Get([]byte(key))

		var newStore map[string]*Store
		if len(b) == 0 {
			// First save of the month!
			newStore = make(map[string]*Store)
		} else {
			if err = json.Unmarshal(b, &newStore); err != nil {
				return ErrUnmarshal
			}
		}

		store := &Store{
			Budget:    cmp.Budget,
			Spendable: oldSpendable,
		}

		newStore[cmp.Id] = store

		if b, err = json.Marshal(&newStore); err != nil {
			return err
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, key, b); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// Save everything except the spendable for last month! It's 0 now muahahaha
	lastMonthStore := &Store{
		Budget:   cmpStore.Budget,
		Leftover: cmpStore.Leftover,
		Spent:    cmpStore.Spent,
		Charges:  cmpStore.Charges,
		Pending:  cmpStore.Pending,
	}

	oldStore[cmp.Id] = lastMonthStore

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&oldStore); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, GetLastMonthBudgetKey(), b); err != nil {
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
	st, err := GetStore(db, cfg, "")
	if err != nil {
		return 0, err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return 0, ErrNotFound
	}

	// Save everything except the spendable! It's 0 now muahahaha
	newStore := &Store{
		Budget:   store.Budget,
		Leftover: store.Leftover,
		Spent:    store.Spent,
		Charges:  store.Charges,
		Pending:  store.Pending,
	}

	st[cmp.Id] = newStore

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		var b []byte
		if b, err = json.Marshal(&st); err != nil {
			return
		}

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, getBudgetKey(), b); err != nil {
			return
		}
		return
	}); err != nil {
		log.Println("Error when saving budget key", err)
		return 0, err
	}

	return store.Spendable, nil
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

		b := tx.Bucket([]byte(cfg.BudgetBuckets.Budget)).Get([]byte(key))
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

func SaveStore(db *bolt.DB, cfg *config.Config, store *Store, cid string) error {
	if err := db.Update(func(tx *bolt.Tx) (err error) {
		key := getBudgetKey()
		b := tx.Bucket([]byte(cfg.BudgetBuckets.Budget)).Get([]byte(key))

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

		if err = misc.PutBucketBytes(tx, cfg.BudgetBuckets.Budget, key, b); err != nil {
			return
		}

		return
	}); err != nil {
		log.Println("Error when saving store", err)
		return err
	}
	return nil
}
