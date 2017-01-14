package subscriptions

import (
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

type Enterprise struct {
}

func (plan *Enterprise) Name() string {
	return "Enterprise"
}

func (plan *Enterprise) IsEligibleInfluencer(inf influencer.Influencer) bool {
	return true
}

func (plan *Enterprise) IsEligibleCampaign(cmp common.Campaign) bool {
	return true
}
