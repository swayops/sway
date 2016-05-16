package common

type Advertiser struct {
	// Id will be assigned by backend
	Id       string `json:"id,omitempty"`
	AgencyId string `json:"agencyId"`

	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`

	ExchangeFee float32 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float32 `json:"dspFee,omitempty"`      // Percentage (decimal)
}
