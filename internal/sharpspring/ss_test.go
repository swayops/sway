package sharpspring

import "testing"
import "github.com/swayops/sway/config"

const (
	utm    = "utm_source=facebook&utm_medium=cpm&utm_campaign=retargeting"
	testID = "0"
	email  = "test4@swayops.com"
)

func init() {
	debug = false
}

func TestCreateLead(t *testing.T) {
	var c config.Config
	c.SharpSpring.AccountID, c.SharpSpring.APIKey = "3_7DF6D4EB282653A5BF4147C8B45E9E04", "CBBCDCDDCF7374AA49C3CE5B0A5A577B"

	if err := CreateLead(&c, testID, "2", "Company Name x", email, utm); err != nil {
		t.Fatal("createerror:", err)
	}

	if err := CreateLead(&c, testID, "2", "Company Name x", email, utm); err == nil {
		t.Fatal("expected error, got nil")
	}
}
