package common

import (
	"github.com/swayops/sway/platforms/facebook"
	"github.com/swayops/sway/platforms/instagram"
	"github.com/swayops/sway/platforms/twitter"
	"github.com/swayops/sway/platforms/youtube"
)

// This deal represents a possible bid
// for an influencer. Do NOT confuse this
// with a Campaign
type Deal struct {
	Id           string `json:"id"`
	CampaignId   string `json:"campaignId"`
	AdvertiserId string `json:"advertiserId"`

	// Platform determined by GetAvailableDeals with value as potential pricepoint
	// This is also saved/reset in the un/assign handlers
	Platforms map[string]float32 `json:"platforms,omitempty"`

	// Timestamp for when the deal was picked up by an influencer
	Assigned int32 `json:"assigned,omitempty"`
	// Timestamp for when the deal was completed by an influencer
	Completed int32 `json:"completed,omitempty"`

	// All of the following are when a deal is assigned/unassigned
	// or times out
	InfluencerId     string  `json:"influencerId,omitempty"`
	AssignedPlatform string  `json:"assignedPlatform,omitempty"`
	AssignedPrice    float32 `json:"assignedPrice,omitempty"`

	// Only set once deal is completed. Contain
	// the information for the post which satisfied the deal
	Tweet     *twitter.Tweet  `json:"tweet,omitempty"`
	Facebook  *facebook.Post  `json:"facebook,omitempty"`
	Instagram *instagram.Post `json:"instagram,omitempty"`
	YouTube   *youtube.Post   `json:"youtube,omitempty"`

	// Requirements copied from the campaign to the deal
	// GetAvailableDeals
	Tags    []string `json:"hashtags,omitempty"`
	Mention string   `json:"mention,omitempty"`
	Link    string   `json:"link,omitempty"`
	Task    string   `json:"task,omitempty"`
	Perks   string   `json:"perks,omitempty"` // Perks need to be specced out
}
