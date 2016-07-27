package common

import "github.com/swayops/sway/platforms/lob"

type Perk struct {
	Name     string `json:"name,omitempty"`
	Category string `json:"category,omitempty"`
	Count    int    `json:"count,omitempty"`

	// Set once a user picks up a deal
	InfId   string           `json:"id,omitempty"`
	InfName string           `json:"infName,omitempty"`
	Address *lob.AddressLoad `json:"address,omitempty"`
	Status  bool             `json:"status,omitempty"`
}