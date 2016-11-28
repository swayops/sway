package sharpspring

import (
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
)

const (
	utm    = "utm_source=facebook&utm_medium=cpm&utm_campaign=retargeting"
	testID = "0"
)

func init() {
	debug = true
}

func TestCreateLead(t *testing.T) {
	var c config.Config
	c.SharpSpring.AccountID, c.SharpSpring.APIKey = "3_7DF6D4EB282653A5BF4147C8B45E9E04", "CBBCDCDDCF7374AA49C3CE5B0A5A577B"

	email := strconv.FormatInt(time.Now().Unix(), 32) + "@swayops.com"

	if err := CreateLead(&c, AdvList, testID, "2", "Company Name x", email, utm); err != nil {
		t.Fatal("createerror:", err)
	}

	if err := CreateLead(&c, AdvList, testID, "2", "Company Name x", email, utm); err == nil {
		t.Fatal("expected error, got nil")
	}

	Post(&c, gin.H{
		"id":     "xxzczxczdasdaz",
		"method": "getActiveLists",
		"params": gin.H{"where": ""},
	})
}
