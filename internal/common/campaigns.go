package common

import "github.com/swayops/sway/misc"

type Campaign struct {
	Id   string `json:"id"` // Should not passed for putCampaign
	Name string `json:"name"`

	Budget float64 `json:"budget"`
	Span   string  `json:"span"` // Timespan the budget represents

	AdvertiserId string `json:"advertiserId"`
	AgencyId     string `json:"agencyId"`

	Active bool `json:"active"`

	// Social Media Post/User Requirements
	Tags    []string          `json:"hashtags,omitempty"`
	Mention string            `json:"mention,omitempty"`
	Link    string            `json:"link,omitempty"`
	Task    string            `json:"task,omitempty"`
	Geos    []*misc.GeoRecord `json:"geos,omitempty"`   // Geos the campaign is targeting
	Gender  string            `json:"gender,omitempty"` // "m" or "f" or "mf"

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`

	// Categories the client is targeting
	Categories []string `json:"categories,omitempty"`

	Perks string `json:"perks,omitempty"` // Perks need to be specced out

	// Internal attribute set by putCampaign and un/assignDeal
	Deals map[string]*Deal `json:"deals,omitempty"`
}
