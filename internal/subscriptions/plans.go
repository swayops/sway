package subscriptions

import (
	"github.com/stripe/stripe-go/sub"
	"github.com/swayops/sway/internal/common"
)

const (
	HYPERLOCAL        = 1
	PREMIUM           = 2
	ENTERPRISE        = 3
	SwayOpsAdAgencyID = "2"
)

type Plan interface {
	Name() string
	IsEligibleInfluencer(followers int64) bool
	IsEligibleCampaign(campaign *common.Campaign) bool
	GetKey(monthly bool) string
}

func CanCampaignRun(isSelfServe bool, subID string, planID int, campaign *common.Campaign) (bool, error) {
	if !isSelfServe {
		return true, nil
	}

	// Checks if the campaign is allowed the given capabilities
	// Lets make sure this subscription is still active!
	active, err := IsSubscriptionActive(true, subID)

	if err != nil {
		return false, err
	}

	if !active {
		// The subscription is no longer active! DEY NOT PAYIN UP
		return false, nil
	}

	plan := GetPlan(planID)
	if plan == nil {
		// They have no plan!
		return false, nil
	}

	return plan.IsEligibleCampaign(campaign), nil
}

func CanInfluencerRun(adAgencyId string, planID int, followers int64) bool {
	if adAgencyId != SwayOpsAdAgencyID {
		// If it's not self serve.. they can do whatever!
		return true
	}

	// Checks if the influencer is allowed to run given the plan
	plan := GetPlan(planID)
	if plan == nil {
		// They have no plan!
		return false
	}

	return plan.IsEligibleInfluencer(followers)
}

func GetPlan(ID int) (p Plan) {
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

func IsSubscriptionActive(selfServe bool, subID string) (bool, error) {
	if !selfServe {
		// Anything that's not self serve can do whatever they want!
		return true, nil
	}

	if subID == "" {
		// No subscription set? That's easy
		return false, nil
	}

	target, err := sub.Get(subID, nil)
	if err != nil {
		return false, err
	}
	if st := target.Status; st == "active" || st == "trialing" {
		return true, nil
	}

	return false, err
}
