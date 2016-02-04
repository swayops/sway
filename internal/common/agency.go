package common

// Agency has an RTB branch with advertisers
// Agency can also create influencer groups, which contain influencers

type Agency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Type             string   `json:"type,omitempty"` // Either "group" OR "rtb"
	RTB              *RTB     `json:"rtb,omitempty"`
	InfluencerGroups []string `json:"influencerGroups,omitempty"`
}

type RTB struct {
	Fee         float32  `json:"fee"` // To be specced out
	Advertisers []string `json:"adv,omitempty"`
}
