package subscriptions

import (
	"strings"

	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/geo"
)

type Premium struct {
}

func (plan *Premium) Name() string {
	return "Premium"
}

func (plan *Premium) GetKey(monthly bool) string {
	// Returns stripe key
	if monthly {
		return "Premium Monthly"
	} else {
		return "Premium Yearly"
	}
}

func (plan *Premium) IsEligibleInfluencer(followers int64) bool {
	// No more than 1 million followers!
	if followers > 1000000 {
		return false
	}

	return true
}

func (plan *Premium) IsEligibleCampaign(cmp *common.Campaign) bool {
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

func (plan *Premium) CanAddSubUser(curr int) bool {
	// No more than 5 subusers!
	if curr >= 5 {
		return false
	}

	return true
}
