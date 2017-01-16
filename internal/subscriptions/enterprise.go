package subscriptions

import "github.com/swayops/sway/internal/common"

type Enterprise struct {
}

func (plan *Enterprise) Name() string {
	return "Enterprise"
}

func (plan *Enterprise) GetKey(monthly bool) string {
	// Returns stripe key
	return ""
}

func (plan *Enterprise) IsEligibleInfluencer(followers int64) bool {
	return true
}

func (plan *Enterprise) IsEligibleCampaign(cmp common.Campaign) bool {
	return true
}
