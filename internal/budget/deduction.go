package budget

const (
	// YouTube
	YT_LIKE    = 0.05
	YT_DISLIKE = 0.03
	YT_VIEW    = 0.0015 // $1.50 CPM
	YT_COMMENT = 0.15

	// Facebook
	FB_LIKE    = 0.03
	FB_SHARE   = 0.3
	FB_COMMENT = 0.15

	// Instagram
	INSTA_LIKE    = 0.03
	INSTA_COMMENT = 0.15

	// Twitter
	TW_RETWEET  = 0.2
	TW_FAVORITE = 0.03
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
