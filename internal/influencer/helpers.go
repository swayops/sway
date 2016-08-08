package influencer

import (
	"fmt"
	"time"
)

const dateFormat = "%d-%02d"

func getDate() string {
	return getDateFromTime(time.Now().UTC())
}

func getDateFromTime(t time.Time) string {
	return fmt.Sprintf(
		dateFormat,
		t.Year(),
		t.Month(),
	)
}

func degradeRep(val int32, rep float64) float64 {
	if val > 0 && val < 5 {
		rep = rep * 0.75
	} else if val >= 5 && val < 20 {
		rep = rep * 0.5
	} else if val >= 20 && val < 50 {
		rep = rep * 0.25
	} else if val >= 50 {
		rep = rep * 0.05
	}
	return rep
}
