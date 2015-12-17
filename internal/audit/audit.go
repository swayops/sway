package audit

import "github.com/swayops/sway/internal/rtb"

// Package used for auditing

func Audit(d *rtb.Deal) *rtb.Deal {
	// Needs to be further specced out
	// Function should mark the deal as audited after some logic
	if d.Completed && !d.Audited {
		// If the deal has not been marked
		// completed by influencer, ignore
		// Insert some logic OR ingest approval from UI
		d.Audited = true
	}
	return d
}
