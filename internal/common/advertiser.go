package common

type Advertiser struct {
	// Id will be assigned by backend
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`

	Name      string   `json:"name"`
	Campaigns []string `json:"campaigns"`

	ExchangeFee float32 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float32 `json:"dspFee,omitempty"`      // Percentage (decimal)
}
