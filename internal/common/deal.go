package common

// This deal represents an outgoing bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id         string `json:"id"`
	CampaignId string `json:"campaignId"`

	Assigned  int32 `json:"assigned, omitempty"`  // Timestamp for when the deal was picked up
	Completed int32 `json:"completed, omitempty"` // Timestamp for when the deal was completed
	Audited   int32 `json:"audited, omitempty"`   // Timestamp for when the deal was audited

	InfluencerId string `json:"infId, omitempty"` // Influencer this deal has been assigned to

	Platforms map[string]float32 `json:"platforms"` // Tmp platform determined by GetAvailableDeals with value as potential pricepoint

	// Requirements added by GetAvailableDeals temporarily for json response
	Tag     string `json:"tag"`
	Mention string `json:"mention"`
	Link    string `json:"link"`
	Task    string `json:"task"`
	Perks   string `json:"perks"` // Perks need to be specced out
}
