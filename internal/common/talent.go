package common

type TalentAgency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	UserId string `json:"userId,omitempty"` // User this belongs to
}
