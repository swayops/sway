package auth

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/common"
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

	// Not required at sign up now..
	// Admin will audit and set gender, geo and categories

	// if inf.Gender != "" && (inf.Gender != "m" && inf.Gender != "f" && inf.Gender != "unicorn") {
	// 	return ErrBadGender
	// }

	// if inf.Geo == nil {
	// 	return ErrNoGeo
	// }

	inf.Categories = common.LowerSlice(inf.Categories)
	for _, cat := range inf.Categories {
		if _, ok := common.CATEGORIES[cat]; !ok {
			return ErrBadCat
		}
	}

	// discussed with Nick and we should allow this for now
	// if len(inf.InstagramId)+len(inf.FbId)+len(inf.TwitterId)+len(inf.YouTubeId) == 0 {
	// 	return ErrPlatform
	// }

	return nil
}

func (inf *InfluencerLoad) setToUser(a *Auth, u *User) error {
	if inf == nil {
		return ErrUnexpected
	}

	if u.ID == "" {
		panic("wtfmate?")
	}

	rinf, err := influencer.New(
		u.ID,
		u.Name,
		inf.TwitterId,
		inf.InstagramId,
		inf.FbId,
		inf.YouTubeId,
		inf.Male,
		inf.Female,
		inf.InviteCode,
		u.ParentID,
		u.Email,
		inf.IP,
		inf.Categories,
		inf.Address,
		int32(u.CreatedAt), // Seconds!
		a.cfg)

	if err != nil {
		return err
	}

	u.ParentID = rinf.AgencyId
	u.InfluencerLoad = nil
	u.Influencer = &Influencer{rinf}

	// Set value to influencer cache
	a.Influencers.SetInfluencer(u.ID, *rinf)

	return err
}
