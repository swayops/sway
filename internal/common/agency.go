package common

type AdAgency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	UserId string `json:"userId,omitempty"` // User this belongs to

	Fee float32 `json:"fee"` // To be specced out
}
