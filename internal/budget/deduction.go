package budget

const (
	// YouTube
	YT_LIKE    = 0.1
	YT_DISLIKE = 0.03
	YT_VIEW    = 0.003 // $3 CPM
	YT_COMMENT = 0.35

	// Facebook
	FB_LIKE    = 0.05
	FB_SHARE   = 0.2
	FB_COMMENT = 0.1

	// Instagram
	INSTA_LIKE    = 0.05
	INSTA_COMMENT = 0.10

	// Twitter
	TW_RETWEET  = 0.2
	TW_FAVORITE = 0.05
)

func (store *Store) deductSpendable(val float32) {
	if store.Spendable <= 0 {
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
