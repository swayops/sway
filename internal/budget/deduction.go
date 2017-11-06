package budget

const (
	// YouTube
	YT_LIKE    = 0.23
	YT_DISLIKE = 0.04
	YT_VIEW    = 0.0015 // $1.50 CPM
	YT_COMMENT = 0.35

	// Facebook
	FB_LIKE    = 0.18
	FB_SHARE   = 0.17
	FB_COMMENT = 0.17

	// Instagram
	INSTA_LIKE    = 0.26
	INSTA_COMMENT = 0.35

	// Twitter
	TW_RETWEET  = 0.2
	TW_FAVORITE = 0.1

	CLICK = 0.6
)

func DeductSpendable(store *Store, val float64) *Store {
	if store.Spendable <= 0 || val <= 0 {
		return store
	}

	if store.Spendable >= val {
		// We have enough spendable to deduct fully
		store.Spent += val
		store.Spendable -= val
	} else {
		// This value goes over our spendable
		store.Spent += store.Spendable
		store.Spendable = 0
	}

	return store
}
