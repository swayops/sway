package common

type Group struct {
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`
	Name     string `json:"name"`

	Influencers []string `json:"influencers"` // Array of influencer IDs in this group
}
