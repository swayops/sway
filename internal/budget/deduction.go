package budget

const (
	// YouTube
	YT_LIKE    = 0.23
	YT_DISLIKE = 0.01
	YT_VIEW    = 0.002 // $3 CPM
	YT_COMMENT = 0.3

	// Facebook
	FB_LIKE    = 0.13
	FB_SHARE   = 0.17
	FB_COMMENT = 0.17

	// Instagram
	INSTA_LIKE    = 0.18
	INSTA_COMMENT = 0.29

	// Twitter
	TW_RETWEET  = 0.2
	TW_FAVORITE = 0.1

	CLICK = 0.9
)

func (store *Store) deductSpendable(val float64) {
	if store.Spendable <= 0 || val <= 0 {
		return
	}

	precalc := store.Spendable - val
	if precalc < 0 {
		store.Spendable = 0
		return
	}

	store.Spendable = precalc
	return
}
