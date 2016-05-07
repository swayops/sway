package common

type TalentAgency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Fee    float32 `json:"fee,omitempty"` // Percentage (decimal)
	Status bool    `json:"status,omitempty"`
}
