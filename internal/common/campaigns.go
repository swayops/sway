package common

import "github.com/swayops/sway/misc"

type Campaign struct {
	Id   string `json:"id"` // Do not pass in putCampaign
	Name string `json:"name"`

	Budget float64 `json:"budget"` // Daily
	Span   string  `json:"span"`   // budget span

	AdvertiserId string `json:"advertiserId"`
	AgencyId     string `json:"agencyId"`

	Active bool `json:"active"`

	// Filters from Advertiser
	Tags    []string `json:"hashtags,omitempty"`
	Mention string   `json:"mention,omitempty"`
	Link    string   `json:"link,omitempty"`
	Task    string   `json:"task,omitempty"`

	GroupIds []string          `json:"groupIds,omitempty"` // Influencer groups the client is targeting
	Geos     []*misc.GeoRecord `json:"geos,omitempty"`     // Geos the campaign is targeting
	Gender   string            `json:"gender,omitempty"`   // "m" or "f" or "mf"

	// Inventory Types Campaign is Targeting
	Twitter   bool `json:"twitter,omitempty"`
	Facebook  bool `json:"facebook,omitempty"`
	Instagram bool `json:"instagram,omitempty"`
	YouTube   bool `json:"youtube,omitempty"`
	// Tumblr    bool `json:"tumblr,omitempty"`

	Perks string `json:"perks,omitempty"` // Perks need to be specced out

	Deals map[string]*Deal `json:"deals,omitempty"`
}
