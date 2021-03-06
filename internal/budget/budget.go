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

func (st *Store) IsClosed(cmp *common.Campaign) bool {
	// Is the store closed for business?
	return st == nil || (st.Spendable <= 0 && !cmp.IsProductBasedBudget())
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

func RemoteBill(tx *bolt.Tx, cfg *config.Config, cmp *common.Campaign, cust string, isIO bool) error {
	// Creates budget keys for NEW campaigns
	b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(cmp.AdvertiserId))

	var (
		st  map[string]*Store
		err error
	)
	if len(b) == 0 {
		// First save for this advertiser!
		st = make(map[string]*Store)
	} else {
		if err = json.Unmarshal(b, &st); err != nil {
			return ErrUnmarshal
		}
	}

	oldStore, ok := st[cmp.Id]
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
}

func Create(tx *bolt.Tx, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string) error {
	// Creates budget keys for NEW campaigns
	b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(cmp.AdvertiserId))

	var (
		st  map[string]*Store
		err error
	)
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
}

func ReplenishSpendable(tx *bolt.Tx, cfg *config.Config, cmp *common.Campaign, isIO bool, cust string) error {
	st, err := GetAdvertiserStore(tx, cfg, cmp.AdvertiserId)
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
		NextBill:  store.NextBill,
	}

	if !isIO && !cmp.IsProductBasedBudget() {
		// CHARGE!
		if cust == "" {
			return ErrCC
		}

		if err := newStore.Bill(cust, spendable, tx, cmp, cfg); err != nil {
			return err
		}
	}

	st[cmp.Id] = newStore

	var b []byte
	if b, err = json.Marshal(&st); err != nil {
		return err
	}

	if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
		return err
	}

	return nil
}

func ClearSpendable(tx *bolt.Tx, cfg *config.Config, cmp *common.Campaign) (float64, error) {
	st, err := GetAdvertiserStore(tx, cfg, cmp.AdvertiserId)
	if err != nil {
		return 0, err
	}

	store, ok := st[cmp.Id]
	if !ok {
		return 0, ErrNotFound
	}

	// Save everything except the spendable! It's 0 now muahahaha
	newStore := &Store{
		Spent:    store.Spent,
		Charges:  store.Charges,
		NextBill: store.NextBill,
	}

	st[cmp.Id] = newStore

	var b []byte
	if b, err = json.Marshal(&st); err != nil {
		return 0, err
	}

	if err = misc.PutBucketBytes(tx, cfg.Bucket.Budget, cmp.AdvertiserId, b); err != nil {
		return 0, err
	}

	return store.Spendable, nil
}

func GetCampaignStore(tx *bolt.Tx, cfg *config.Config, cid, advid string) (*Store, error) {
	st, err := GetAdvertiserStore(tx, cfg, advid)
	if err != nil {
		return nil, err
	}
	if store, ok := st[cid]; ok {
		return store, nil
	}
	return nil, ErrNotFound
}

func GetCampaignStoreFromDb(db *bolt.DB, cfg *config.Config, cid, advid string) (*Store, error) {
	var (
		budgetStore *Store
	)
	if err := db.View(func(tx *bolt.Tx) (err error) {
		budgetStore, err = GetCampaignStore(tx, cfg, cid, advid)
		if err != nil {
			return err
		}
		return nil
	}); err != nil || budgetStore == nil {
		return nil, ErrNotFound
	}
	return budgetStore, nil
}

func GetAdvertiserStore(tx *bolt.Tx, cfg *config.Config, advID string) (map[string]*Store, error) {
	// Gets budget store keyed off of Campaign ID for a given month
	var st map[string]*Store

	b := tx.Bucket([]byte(cfg.Bucket.Budget)).Get([]byte(advID))
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, ErrUnmarshal
	}

	return st, nil
}

func GetStore(tx *bolt.Tx, cfg *config.Config) (map[string]*Store, error) {
	// Gets budget store keyed off of Campaign ID for a given month
	store := make(map[string]*Store)
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

	return store, nil
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
