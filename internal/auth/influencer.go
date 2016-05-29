package auth

import (
	"encoding/json"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
	"github.com/swayops/sway/internal/influencer"
)

type Influencer struct {
	influencer.Influencer
}

func GetInfluencer(u *User) *Influencer {
	if u == nil || u.Type != InfluencerScope {
		return nil
	}
	var inf Influencer
	if json.Unmarshal(u.Data, &inf) != nil || inf.Gender == "" {
		return nil
	}
	inf.Id, inf.AgencyId, inf.Name = u.ID, u.ParentID, u.Name
	return &inf
}

func (a *Auth) GetInfluencerTx(tx *bolt.Tx, curUser *User, userID string) *Influencer {
	if curUser != nil && curUser.ID == userID {
		return GetInfluencer(curUser)
	}
	return GetInfluencer(a.GetUserTx(tx, userID))
}

func (inf *Influencer) Check() error {
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

func (inf *Influencer) setToUser(a *Auth, u *User) error {
	if inf.Name != "" {
		u.Name = inf.Name
	}
	inf.Id, inf.AgencyId, inf.Name = "", "", "" // this is a part of the user struct
	j, err := json.Marshal(inf)
	u.Data = j
	return err
}

func getInfluencerLoad(u *User) *InfluencerLoad {
	if u.Type != InfluencerScope {
		return nil
	}
	var inf InfluencerLoad
	if json.Unmarshal(u.Data, &inf) != nil || inf.Gender == "" {
		return nil
	}
	return &inf
}

type InfluencerLoad struct {
	influencer.InfluencerLoad
}

func (inf *InfluencerLoad) Check() error {
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

func (inf *InfluencerLoad) setToUser(a *Auth, u *User) error {
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
