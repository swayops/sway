package common

import "time"

func GetMonthOffset(offset int) string {
	t := time.Now().UTC()
	t = time.Date(t.Year(), t.Month()-time.Month(offset), 1, 0, 0, 0, 0, time.UTC)
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

func GetDateRangeFromOffsetRange(start, end int) []time.Time {
	if start == -1 {
		start = -365
	} else if start > 0 {
		start = -start
	}
	if end == -1 {
		end = -365
	} else if end > 0 {
		end = -end
	}

	if end < start {
		start, end = end, start
	}

	out := make([]time.Time, (end-start)+1)
	to := time.Now().UTC().AddDate(0, 0, start)
	for i := range out {
		out[i] = to.AddDate(0, 0, i+1)
	}

	return out
}
