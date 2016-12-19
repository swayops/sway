package instagram

import (
	"github.com/swayops/sway/config"
)

func Status(cfg *config.Config) bool {
	if id, err := New("instagram", cfg); err != nil || id == nil || id.Followers == 0 {
		return false
	}
	return true
}
