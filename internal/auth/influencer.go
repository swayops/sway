package auth

import (
	"strings"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/influencer"
)

type Influencer struct {
	*influencer.Influencer
}

func GetInfluencer(u *User) *Influencer {
	if u == nil || u.Influencer == nil {
		return nil
	}
	inf := u.Influencer
	if inf.Influencer == nil { // using a pointer here so we wouldn't create a copy on assignment in setToUser
		return nil
	}
	return inf
}

func (a *Auth) GetInfluencerTx(tx *bolt.Tx, userID string) *Influencer {
	return GetInfluencer(a.GetUserTx(tx, userID))
}

func (a *Auth) GetInfluencer(userID string) (inf *Influencer) {
	a.db.View(func(tx *bolt.Tx) error {
		inf = GetInfluencer(a.GetUserTx(tx, userID))
		return nil
	})
	return
}

func (inf *Influencer) Check() error { return nil } // this is to fulfill the interface
func (inf *Influencer) setToUser(_ *Auth, u *User) error {
	u.Influencer = inf
	return nil
}

type InfluencerLoad struct {
	influencer.InfluencerLoad
}

func (inf *InfluencerLoad) Check() error {
	if inf == nil {
		return ErrUnexpected
	}

	if inf.Name != "" && len(strings.Split(inf.Name, " ")) < 2 {
		return ErrName
	}

	// Not required at sign up now..
	// Admin will audit and set these

	// if inf.Gender != "m" && inf.Gender != "f" && inf.Gender != "unicorn" {
	// 	return ErrBadGender
	// }

	// if inf.Geo == nil {
	// 	return ErrNoGeo
	// }

	// inf.Categories = common.LowerSlice(inf.Categories)
	// for _, cat := range inf.Categories {
	// 	if _, ok := common.CATEGORIES[cat]; !ok {
	// 		return ErrBadCat
	// 	}
	// }

	if len(inf.InstagramId)+len(inf.FbId)+len(inf.TwitterId)+len(inf.YouTubeId) == 0 {
		return ErrPlatform
	}

	return nil
}

func (inf *InfluencerLoad) setToUser(a *Auth, u *User) error {
	if inf == nil {
		return ErrUnexpected
	}

	if u.ID == "" {
		panic("wtfmate?")
	}

	if inf.Name == "" {
		inf.Name = u.Name
	} else {
		u.Name = inf.Name
	}

	rinf, err := influencer.New(
		u.ID,
		inf.Name,
		inf.TwitterId,
		inf.InstagramId,
		inf.FbId,
		inf.YouTubeId,
		inf.Gender,
		inf.InviteCode,
		u.ParentID,
		inf.Categories,
		inf.Geo,
		inf.Address,
		a.cfg)

	if err != nil {
		return err
	}

	u.InfluencerLoad = nil
	u.Influencer = &Influencer{rinf}

	return err
}
