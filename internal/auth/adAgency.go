package auth

import "github.com/boltdb/bolt"

type AdAgency struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func GetAdAgency(u *User) *AdAgency {
	if u == nil || u.Type != AdAgencyScope {
		return nil
	}
	return &AdAgency{
		ID:     u.ID,
		Name:   u.Name,
		Status: u.Status,
	}
}

func (a *Auth) GetAdAgencyTx(tx *bolt.Tx, curUser *User, userID string) *AdAgency {
	if curUser != nil && curUser.ID == userID {
		return GetAdAgency(curUser)
	}
	return GetAdAgency(a.GetUserTx(tx, userID))
}

func (ag *AdAgency) setToUser(_ *Auth, u *User) error {
	if ag.Name != "" {
		u.Name, u.Status = ag.Name, ag.Status
	}
	return nil
}

func (ag *AdAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	return nil
}
