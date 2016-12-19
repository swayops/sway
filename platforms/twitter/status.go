package twitter

import (
	"github.com/swayops/sway/config"
)

func Status(cfg *config.Config) bool {
	if id, err := New("twitter", cfg); err != nil || id == nil || id.Followers == 0 {
		return false
	}

	return true
}
