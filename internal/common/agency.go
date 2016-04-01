package common

type AdAgency struct {
	// Id will be assigned by backend
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	UserId string `json:"userId,omitempty"` // User this belongs to

	Fee float32 `json:"fee,omitempty"` // To be specced out
}
