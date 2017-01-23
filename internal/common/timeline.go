package common

const (
	// Startup message for non-perk campaigns
	CAMPAIGN_APPROVAL = "Campaign is currently awaiting approval from Sway."
	CAMPAIGN_START    = "Sway is currently deciding which influencers will be activated on behalf of your campaign."

	// Startup messages for perk campaigns
	PERK_WAIT      = "We are awaiting delivery of your perks before your campaign can start."
	PERKS_RECEIVED = "Sway has received your perk shipment and is currently deciding which influencers will be activated on behalf of your campaign."

	// Standard messages
	DEAL_ACCEPTED    = "Influencers have accepted your campaign and are now making content."
	PERKS_MAILED     = "Perks have been shipped to influencers."
	CAMPAIGN_SUCCESS = "Social posts have been made!"

	CAMPAIGN_PAUSED = "Campaign has been paused!"
)

type Timeline struct {
	Message string `json:"msg,omitempty"`
	TS      int64  `json:"ts,omitempty"`
	Link    string `json:"link,omitempty"`
}
