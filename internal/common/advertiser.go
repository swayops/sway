package common

type Advertiser struct {
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`

	Name      string   `json:"name"`
	Campaigns []string `json:"campaigns"`
}
