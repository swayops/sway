package subscriptions

import (
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

const (
HYPERLOCAL = "0"
PREMIUM = "1"
ENTERPRISE = "2")

type Plan interface {
	Name() string
	IsEligibleInfluencer(inf influencer.Influencer) bool
	IsEligibleCampaign(cmp common.Campaign) bool
	GetKey() string
}

func CanCampaignRun(agencyID string, planID string, cmp Common.Campaign) bool {
	// Checks if the campaign is allowed the given capabilities

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
	case HYPERLOCAL:
		return new(HyperLocal)
	case PREMIUM:
		return new(Premium)
	case ENTERPRISE:
		return new(Enterprise)
	default:
		return nil
	}
}
