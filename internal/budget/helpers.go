package budget

import "time"

const format = "01-2006"

func getBudgetKey() string {
	return time.Now().UTC().Format(format)
}

func GetLastMonthBudgetKey() string {
	return getBudgetKeyOffset(1)
}

func getBudgetKeyOffset(offset int) string {
	now := time.Now().UTC()
	if offset > 0 {
		offset = -offset
	}
	return now.AddDate(0, offset, 0).Format(format)
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
