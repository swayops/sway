package budget

import (
	"fmt"
	"time"
)

const (
	format = "%d-%02d"
)

func getBudgetKey() string {
	now := time.Now().UTC()

	return fmt.Sprintf(
		format,
		now.Month(),
		now.Year(),
	)
}

func GetLastMonthBudgetKey() string {
	return getBudgetKeyOffset(1)
}

func getBudgetKeyOffset(offset int) string {
	now := time.Now().UTC()

	lastMonth := int(now.Month()) - offset
	year := now.Year()
	if lastMonth == 0 {
		year = year - 1
		lastMonth = 12
	} else if lastMonth < 0 {
		lastMonth = -lastMonth
		lastMonth = 12 - lastMonth
		year = year - 1

	}
	return fmt.Sprintf(
		format,
		lastMonth,
		year,
	)
}

func isFirstDay() bool {
	// Checks to see if today is the first
	// day of the month
	now := time.Now().UTC()
	return now.Day() == 1
}

func daysInMonth(year int, month time.Month) int {
	if month == time.February {
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) { // leap year
			return 29
		}
		return 28
	}

	if month <= 7 {
		month++
	}
	if month&0x0001 == 0 {
		return 31
	}
	return 30
}
