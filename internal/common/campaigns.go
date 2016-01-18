package common

type Campaign struct {
	Id           string  `json:"id"` // Do not pass in a campaign
	Name         string  `json:"name"`
	Budget       float64 `json:"budget"` // Monthly
	AdvertiserId string  `json:"advertiserId"`
	AgencyId     string  `json:"agencyId"`

	Active bool `json:"active"`

	// Filters from Advertiser
	Tag        string   `json:"tag"`
	Mention    string   `json:"mention"`
	Link       string   `json:"link"`
	Task       string   `json:"task"`
	Categories []string `json:"cats"` // Influencer categories client would like to use

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter"`
	Facebook  bool `json:"facebook"`
	Instagram bool `json:"instagram"`
	YouTube   bool `json:"youtube"`
	Tumblr    bool `json:"tmblr"`

	Perks string `json:"perks"` // Perks need to be specced out

	Deals map[string]*Deal `json:"deals"`
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

func (cmp *Campaign) CreateDeal() []*Deal {
	return nil
}

func (cmp *Campaign) DeleteDeal() []*Deal {
	// Remove from bucket AND
	return nil
}

func (cmp *Campaign) UpdateDeal() []*Deal {
	return nil
}
