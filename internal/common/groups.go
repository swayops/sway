package common

// Influencer groups belong to an agency!
type Group struct {
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`
	Name     string `json:"name"`

	Influencers []string `json:"influencers"` // Array of influencers in this group
}
