package auth

import "encoding/json"

type Advertiser struct {
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`

	ExchangeFee float64 `json:"exchangeFee,omitempty"` // Percentage (decimal)
	DspFee      float64 `json:"dspFee,omitempty"`      // Percentage (decimal)
}

func GetAdvertiser(u *User) *Advertiser {
	if u.Type != AdvertiserScope {
		return nil
	}
	var adv Advertiser
	if json.Unmarshal(u.Data, &adv) != nil || adv.ExchangeFee == 0 {
		return nil
	}
	adv.Name, adv.Status = u.Name, u.Status
	return &adv
}

func (adv *Advertiser) setToUser(_ *Auth, u *User) error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.Name != "" {
		u.Name, u.Status = adv.Name, adv.Status
	}
	adv.Name, adv.Status = "", false
	b, err := json.Marshal(adv)
	u.Data = b
	return err
}

func (adv *Advertiser) Check() error {
	if adv == nil {
		return ErrUnexpected
	}

	if adv.ExchangeFee == 0 || adv.ExchangeFee > 0.99 {
		return ErrInvalidFee
	}

	if adv.DspFee == 0 || adv.DspFee > 0.99 {
		return ErrInvalidFee
	}

	return nil
}
