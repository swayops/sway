package facebook

import "time"

const FacebookTimeLayout = `"2006-01-02T15:04:05-0700"`

type FbTime struct {
	time.Time
}

func (t *FbTime) UnmarshalJSON(b []byte) (err error) {
	t.Time, err = time.Parse(FacebookTimeLayout, string(b))
	return
}

func (t *FbTime) MarshalJSON() ([]byte, error) {
	return []byte(t.Format(FacebookTimeLayout)), nil
}
