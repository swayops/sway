package common

type Campaign struct {
	Id     string  `json:"id"`
	Name   string  `json:"name"`
	Budget float64 `json:"budget"` // Weekly.. monthly?

	// Filters from Advertiser
	Tag        string   `json:"tag"`
	Mention    string   `json:"mention"`
	Link       string   `json:"link"`
	Categories []string `json:"cats"` // Influencer categories client would like to use

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter"`
	Facebook  bool `json:"fb"`
	Instagram bool `json:"insta"`
	YouTube   bool `json:"yt"`

	Perks string `json:"perks"` // Perks need to be specced out
}

func (cmp *Campaign) GetActiveDeals() []*Deal {
	// Look at:
	// - currently accepted deals by influencers (and their timeouts)
	// - budget
	// - available influencers
	// - campagin (gender, category, geo) filters
	// - stats for each influencer using stores social media stats (to determine deal price)
	// and return optimized deals for this campaign

	// A ticker should regularly call this function. For any
	// influencers who have fallen out of deal requirements OR
	// hit the post timeout (post must be made within 1 day) will be notified that
	// they are no longer eligible. Also, new influencers who are eligible
	// (assuming campaign has budget), will be notified that a new deal is available

	return nil
}

func (cmp *Campaign) GetCompletedDeals() []*Deal {
	// Return all deals that have been completed
	// and audited

	return nil
}

func (cmp *Campaign) ReserveDeal() []*Deal {
	// Track:
	// - all influencers who have previously been notified of a deal and accepted

	// This function should have a corresponding handler
	// which allows for ingesting a newly approved deal from the app
	return nil
}
