package misc

type GeoRecord struct {
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Country string `json:"country,omitempty"`
	Zip     string `json:"zip,omitempty"`

	Latitude   float64 `json:"lat,omitempty"`
	Longtitude float64 `json:"long,omitempty"`
}
