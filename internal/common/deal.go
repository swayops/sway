package common

import (
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

// This deal represents an outgoing bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id           string `json:"id"`
	CampaignId   string `json:"campaignId"`
	AdvertiserId string `json:"advertiserId"`

	InfluencerId string `json:"influencerId,omitempty"` // Influencer this deal has been assigned to

	// Platform determined by GetAvailableDeals with value as potential pricepoint
	// This is also saved/reset in the un/assign handlers
	Platforms map[string]float32 `json:"platforms,omitempty"`

	Assigned  int32 `json:"assigned,omitempty"`  // Timestamp for when the deal was picked up
	Completed int32 `json:"completed,omitempty"` // Timestamp for when the deal was completed

	// Both saved/reset in the un/assignDeal method
	AssignedPlatform string  `json:"assignedPlatform,omitempty"`
	AssignedPrice    float32 `json:"assignedPrice,omitempty"`

	// Only set once deal is completed
	Tweet     *twitter.Tweet  `json:"tweet,omitempty"`
	Facebook  *facebook.Post  `json:"facebook,omitempty"`
	Instagram *instagram.Post `json:"instagram,omitempty"`
	YouTube   *youtube.Post   `json:"youtube,omitempty"`

	// Requirements added by GetAvailableDeals for json response
	// for get deals accessed by influencers (so they know requirements)
	// This is also saved in un/assignDeal
	Tags    []string `json:"hashtags,omitempty"`
	Mention string   `json:"mention,omitempty"`
	Link    string   `json:"link,omitempty"`
	Task    string   `json:"task,omitempty"`
	Perks   string   `json:"perks,omitempty"` // Perks need to be specced out
}
