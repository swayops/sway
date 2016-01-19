package common

// This deal represents an outgoing bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id         string `json:"id"`
	CampaignId string `json:"campaignId"`

	Assigned  int32 `json:"assigned,omitempty"`  // Timestamp for when the deal was picked up
	Completed int32 `json:"completed,omitempty"` // Timestamp for when the deal was completed
	Audited   int32 `json:"audited,omitempty"`   // Timestamp for when the deal was audited

	InfluencerId string `json:"influencerId,omitempty"` // Influencer this deal has been assigned to

	Platforms map[string]float32 `json:"platforms,omitempty"` // Tmp platform determined by GetAvailableDeals with value as potential pricepoint

	// Requirements added by GetAvailableDeals temporarily for json response
	// for get deals accessed by influencers (so they know requirements)
	Tag     string `json:"tag,omitempty"`
	Mention string `json:"mention,omitempty"`
	Link    string `json:"link,omitempty"`
	Task    string `json:"task,omitempty"`
	Perks   string `json:"perks,omitempty"` // Perks need to be specced out
}
