package common

type Advertiser struct {
	// Id will be assigned by backend
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`

	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)
}
