package subscriptions

import (
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

type Plan interface {
	Name() string
	IsEligibleInfluencer(inf influencer.Influencer) bool
	IsEligibleCampaign(cmp common.Campaign) bool
}

func CanRun(agencyID string, planID string, cmp Common.Campaign) bool {
	if agencyID != auth.SwayOpsTalentAgencyID {
		// Anything that's not Sway can do whatever the hell
		// they want
		return true
	}

	plan := GetPlan(planID)
	if plan == nil {
		// They have no plan!
		return false
	}

	return plan.IsEligibleCampaign(cmp)
}

func GetPlan(ID string) *Plan {
	switch ID {
	case "0":
		return new(HyperLocal)
	case "1":
		return new(Premium)
	case "2":
		return new(Enterprise)
	default:
		return nil
	}
}
