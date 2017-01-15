package subscriptions

import (
	"strings"

	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
	"github.com/swayops/sway/internal/influencer"
)

type Premium struct {
}

func (plan *Premium) Name() string {
	return "Premium"
}

func (plan *Premium) GetKey() string {
	// Returns stripe key
	return "Premium key"
}

func (plan *Premium) IsEligibleInfluencer(inf influencer.Influencer) bool {
	// No more than 1 million followers!
	if inf.GetFollowers() > 1000000 {
		return false
	}

	return true
}

func (plan *Premium) IsEligibleCampaign(cmp common.Campaign) bool {
	// USA, Canada, and EU targeting only!
	for _, cGeo := range cmp.Geos {
		cy := strings.ToLower(cGeo.Country)
		_, isEU := geo.EU_COUNTRIES[cy]
		if cy != "us" && cy != "ca" && !isEU {
			return false
		}
	}

	return true
}
