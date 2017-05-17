package budget

import (
	"fmt"
	"time"
)

const format = "01-2006"

func GetSpendHistoryKey() string {
	now := time.Now()
	last := now.AddDate(0, -1, 0)

	return fmt.Sprintf("%s-%s", last.Format("20060102"), now.Format("20060102"))
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

const (
	DEFAULT_DSP_FEE      = 0.2
	DEFAULT_EXCHANGE_FEE = 0.2
	DEFAULT_AGENCY_FEE   = 0.2
)

func GetMargins(total, dspFee, exchangeFee, agencyFee float64) (dsp, exchange, agency, influencer float64) {
	if dspFee == -1 {
		dspFee = DEFAULT_DSP_FEE
	}

	if exchangeFee == -1 {
		exchangeFee = DEFAULT_EXCHANGE_FEE
	}

	if agencyFee == -1 {
		agencyFee = DEFAULT_AGENCY_FEE
	}

	// DSP and Exchange fee taken away from the prinicpal
	dsp = total * dspFee
	exchange = total * exchangeFee

	// Talent agency payout will be taken away from the influencer portion
	influencerPool := total - (dsp + exchange)
	agency = influencerPool * agencyFee
	influencer = influencerPool - agency
	return

}
