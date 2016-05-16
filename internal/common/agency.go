package common

type AdAgency struct {
	// Id will be assigned by backend
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Status bool `json:"status,omitempty"`
}
