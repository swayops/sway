package common

type Agency struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Type         string        `json:"type,omitempty"` // Either "group" OR "rtb"
	RTB          *RTB          `json:"rtb,omitempty"`
	TalentAgency *TalentAgency `json:"talentAgency,omitempty"`
}

type RTB struct {
	Fee         float32  `json:"fee"` // To be specced out
	Advertisers []string `json:"adv,omitempty"`
}

type TalentAgency struct {
	Fee    float32  `json:"fee"`              // To be specced out
	Groups []string `json:"groups,omitempty"` // Influencer groups attached to this campaign
}
