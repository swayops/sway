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

const (
	tlColorRed   = `#c0392b`
	tlColorGreen = `#2ecc71`
	tlColorBlue  = `#3498db`
	tlColorGrey  = `#bdc3c7`
)

type Timeline struct {
	Message   string `json:"msg,omitempty"`
	TS        int64  `json:"ts,omitempty"`
	Link      string `json:"link,omitempty"`
	LinkTitle string `json:"linkTitle,omitempty"`
	Color     string `json:"color,omitempty"`
}

func SetLinkTitles(m map[string]*Timeline) {
	for _, tl := range m {
		switch tl.Message {
		case PERK_WAIT:
			tl.LinkTitle = "Learn More »"
			tl.Color = tlColorRed
		case CAMPAIGN_START, PERKS_RECEIVED:
			tl.LinkTitle = "Learn More »"
			tl.Color = tlColorBlue
		case DEAL_ACCEPTED, PERKS_MAILED:
			tl.LinkTitle = "View Campaigns »"
			tl.Color = tlColorBlue
		case CAMPAIGN_SUCCESS:
			tl.LinkTitle = "See Who »"
			tl.Color = tlColorGreen
		case CAMPAIGN_PAUSED:
			tl.LinkTitle = "Edit Campaign »"
			tl.Color = tlColorGrey
		case CAMPAIGN_APPROVAL:
			tl.Color = tlColorRed
		default:
			tl.Color = tlColorGrey
		}
	}
}
