package auth

type AdAgency struct {
	Name   string `json:"name,omitempty"`
	Status bool   `json:"status,omitempty"`
}

func GetAdAgency(u *User) *AdAgency {
	if u.Type != AdAgencyScope {
		return nil
	}
	return &AdAgency{
		Name:   u.Name,
		Status: u.Status,
	}
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
