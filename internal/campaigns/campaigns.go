package campaigns

import "github.com/swayops/sway/internal/deal"

type Campaign struct {
	Id     string
	Name   string
	Budget float64 // Monthly

	// Requirements from advertiser
	Tag        string
	Mention    string
	Link       string
	Categories []string // Influencer categories client would like to use

	// Inventory Types
	Twitter   bool
	Facebook  bool
	Instagram bool

	Perks string // Perks need to be specced out
}

func (cmp *Campaign) PendingDeals() []*deal.Deal {
	// Look at:
	// - currently accepted deals by influencers (and their timeouts)
	// - budget
	// - available influencers
	// - gender and geo filters
	// - stats for each influencer using stores social media stats
	// and return available deals
	// - influencer category filters
	// and return optimized deals for this campaign

	// A ticker should regularly call this function. For any
	// influencers who have fallen out of deal requirements OR
	// hit the post timeout (post must be made within 1 day) will be notified that
	// they are no longer eligible. Also, new influencers who are eligible
	// (assuming campaign has budget), will be notified that a new deal is available

	return nil
}

func (cmp *Campaign) AcceptDeal() []*deal.Deal {
	// Track:
	// - all influencers who have previously been notified of a deal and accepted

	// This function should have a corresponding handler
	// which allows for ingesting approved deals from the app
	return nil
}
