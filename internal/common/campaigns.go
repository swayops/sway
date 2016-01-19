package common

type Campaign struct {
	Id           string  `json:"id"` // Do not pass in a campaign
	Name         string  `json:"name"`
	Budget       float64 `json:"budget"` // Monthly
	AdvertiserId string  `json:"advertiserId"`
	AgencyId     string  `json:"agencyId"`

	Active bool `json:"active"`

	// Filters from Advertiser
	Tag        string   `json:"tag,omitempty"`
	Mention    string   `json:"mention,omitempty"`
	Link       string   `json:"link,omitempty"`
	Task       string   `json:"task,omitempty"`
	Categories []string `json:"cats,omitempty"` // Influencer categories client would like to use

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`
	Tumblr    bool `json:"tumblr,omitempty"`

	Perks string `json:"perks,omitempty"` // Perks need to be specced out

	Deals map[string]*Deal `json:"deals,omitempty"`
}

func (cmp *Campaign) GetAllActiveDeals() []*Deal {
	// Get all deals that are currently assigned to an influencer
	return nil
}

func (cmp *Campaign) GetCompletedDeals() []*Deal {
	// Return all deals that have been completed
	// and audited for this campaign

	return nil
}
