package common

import "github.com/swayops/sway/platforms/lob"

type Perk struct {
	Name  string   `json:"name,omitempty"`
	Type  int      `json:"type,omitempty"`  // 1 = product, 2 = coupon
	Codes []string `json:"codes,omitempty"` // List of coupon codes that are available

	Category     string `json:"category,omitempty"`     // Set internally
	Count        int    `json:"count,omitempty"`        // Set internally for coupons
	PendingCount int    `json:"pendingCount,omitempty"` // Set when the campaign increases perks
	Instructions string `json:"instructions,omitempty"` // Optional

	// Set once a user picks up a deal.. only set for the
	// common.Deal.Perk value! not for the campaign.Perks one
	// since it's set on a per deal basis!
	InfId   string           `json:"id,omitempty"`
	InfName string           `json:"infName,omitempty"`
	Address *lob.AddressLoad `json:"address,omitempty"`
	Status  bool             `json:"status,omitempty"`
	Code    string           `json:"code,omitempty"`
}

func (p *Perk) GetType() string {
	if p.IsCoupon() {
		return "Coupon"
	} else {
		return "Product"
	}
}

func (p *Perk) IsProduct() bool {
	return p.Type == 1
}

func (p *Perk) IsCoupon() bool {
	return p.Type == 2
}
