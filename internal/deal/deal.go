package deal

// This deal represents an outgoing bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id         string
	CampaignId string // Campaign this deal belongs to
	Influencer string // Influencer ID that has taken on the deal
	Completed  bool   // Has the deal been completed/audited?

	Price float64 // Price determined for this influencer using our algo

	// Requirements from advertiser
	Tag     string
	Mention string
	Link    string
	Task    string

	// Only one of these should be true
	Twitter   bool
	Facebook  bool
	Instagram bool
}
