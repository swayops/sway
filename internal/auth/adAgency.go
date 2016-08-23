package auth

import "github.com/boltdb/bolt"

type AdAgency struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func GetAdAgency(u *User) *AdAgency {
	if u == nil {
		return nil
	}
	return u.AdAgency
}

func (a *Auth) GetAdAgencyTx(tx *bolt.Tx, userID string) *AdAgency {
	return GetAdAgency(a.GetUserTx(tx, userID))
}

func (a *Auth) GetAdAgency(userID string) (ag *AdAgency) {
	a.db.View(func(tx *bolt.Tx) error {
		ag = GetAdAgency(a.GetUserTx(tx, userID))
		return nil
	})
	return
}

func (ag *AdAgency) setToUser(_ *Auth, u *User) error {
	// Newly created/updated user is passed in
	if ag == nil {
		return ErrUnexpected
	}
	if u.ID == "" {
		panic("wtfmate?")
	}

	if ag.ID == "" || ag.Name == "" {
		// Initial creation:
		// Copy the newly created user's name and status to
		// the agency
		ag.Name, ag.Status = u.Name, u.Status
	} else if ag.ID != u.ID {
		return ErrInvalidID
	} else if ag.Name != "" {
		// Update the user properties when the
		// agency has been updated
		u.Name, u.Status = ag.Name, ag.Status
	}

	// Make sure IDs are congruent each create/update
	ag.ID = u.ID
	u.AdAgency = ag
	return nil
}

func (ag *AdAgency) Check() error {
	if ag == nil {
		return ErrUnexpected
	}

	return nil
}
