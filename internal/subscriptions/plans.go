package subscriptions

import "github.com/swayops/sway/internal/common"

const (
	HYPERLOCAL            = 1
	PREMIUM               = 2
	ENTERPRISE            = 3
	SwayOpsTalentAgencyID = "3"
)

type Plan interface {
	Name() string
	IsEligibleInfluencer(followers int64) bool
	IsEligibleCampaign(cmp common.Campaign) bool
	GetKey(monthly bool) string
}

func CanCampaignRun(agencyID string, planID int, cmp Common.Campaign) bool {
	// Checks if the campaign is allowed the given capabilities

	if agencyID != SwayOpsTalentAgencyID {
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

func GetPlan(ID int) *Plan {
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
