package subscriptions

import (
	"strings"

	"github.com/swayops/sway/internal/common"
)

type HyperLocal struct {
}

func (plan *HyperLocal) Name() string {
	return "Hyper Local"
}

func (plan *HyperLocal) GetKey(monthly bool) string {
	// Returns stripe key
	if monthly {
		return "Hyper Local Monthly"
	} else {
		return "Hyper Local Yearly"
	}
}

func (plan *HyperLocal) IsEligibleInfluencer(followers int64) bool {
	// No more than 50k followers!
	if followers > 50000 {
		return false
	}

	return true
}

func (plan *HyperLocal) IsEligibleCampaign(cmp *common.Campaign) bool {
	// Coupon perks only!
	if cmp.Perks != nil {
		if !cmp.Perks.IsCoupon() {
			return false
		}
	}

	// USA targeting only!
	for _, geo := range cmp.Geos {
		if strings.ToLower(geo.Country) != "us" {
			return false
		}
	}

	// Instagram targeting only!
	if cmp.Facebook || cmp.Twitter || cmp.YouTube {
		return false
	}

	return true
}

func (plan *HyperLocal) CanAddSubUser(curr int) bool {
	// Only allowed 1!
	return false
}
