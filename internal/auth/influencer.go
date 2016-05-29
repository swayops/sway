package auth

import (
	"encoding/json"

	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

type Influencer struct {
	influencer.Influencer
}

func GetInfluencer(u *User) *Influencer {
	if u.Type != InfluencerScope {
		return nil
	}
	var inf Influencer
	if json.Unmarshal(u.Data, &inf) != nil || inf.Gender == "" {
		return nil
	}
	return &inf
}

func getInfluencerLoad(u *User) *influencerLoad {
	if u.Type != InfluencerScope {
		return nil
	}
	var inf influencerLoad
	if json.Unmarshal(u.Data, &inf) != nil || inf.Gender == "" {
		return nil
	}
	return &inf
}

type influencerLoad struct {
	influencer.InfluencerLoad
}

func (inf *influencerLoad) Check() error {
	if inf == nil {
		return ErrUnexpected
	}
	if inf.Gender != "m" && inf.Gender != "f" && inf.Gender != "unicorn" {
		return ErrBadGender
	}

	if inf.Geo == nil {
		return ErrNoGeo
	}

	inf.Categories = common.LowerSlice(inf.Categories)
	for _, cat := range inf.Categories {
		if _, ok := common.CATEGORIES[cat]; !ok {
			return ErrBadCat
		}
	}

	return nil
}

func (inf *influencerLoad) setToUser(a *Auth, u *User) error {
	if inf.Name == "" {
		inf.Name = u.Name
	}
	rinf, err := influencer.New(
		inf.Name,
		inf.TwitterId,
		inf.InstagramId,
		inf.FbId,
		inf.YouTubeId,
		inf.Gender,
		inf.InviteCode,
		SwayOpsTalentAgencyID,
		inf.Categories,
		inf.Geo,
		a.cfg)

	if err != nil {
		return err
	}
	j, err := json.Marshal(rinf)
	u.Data = j
	return err
}
