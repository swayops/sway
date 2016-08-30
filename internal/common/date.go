package common

import "time"

func GetMonthOffset(offset int) string {
	t := time.Now().UTC()
	t = t.AddDate(0, -offset, 0)
	return t.Format("2006-01")
}

func GetDate() string {
	return GetDateFromTime(time.Now().UTC())
}

func GetDateFromTime(t time.Time) string {
	return t.Format("2006-01-02")
}

func GetDateRange(from, to time.Time) []string {
	out := []string{}
	diff := to.Sub(from).Hours() / 24

	for i := 0; i <= int(diff); i++ {
		out = append(out, GetDateFromTime(from.AddDate(0, 0, i)))
	}
	return out
}

func GetDateRangeFromOffset(off int) []time.Time {
	to := time.Now().UTC()
	if off == -1 {
		off = -365
	} else if off > 0 {
		off = -off
	}
	out := make([]time.Time, -off+1)
	for i := range out {
		out[i] = to.AddDate(0, 0, off+i)
	}
	return out
}
