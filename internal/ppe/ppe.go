package ppe

import (
	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/influencer"
	"github.com/swayops/sway/platforms"
)

var minPpe = float32(0.03) // 3 cents

func Calculate(db *bolt.DB, cfg *config.Config, inf *influencer.Influencer, network string) float32 {
	if inf.FloorPrice > 0 {
		return inf.FloorPrice
	}

	switch network {
	case platform.Twitter:
		return getPpeFromScore(inf.Twitter.GetScore())
	case platform.Facebook:
		return getPpeFromScore(inf.Facebook.GetScore())
	case platform.Instagram:
		return getPpeFromScore(inf.Instagram.GetScore())
	case platform.YouTube:
		return getPpeFromScore(inf.YouTube.GetScore())
	}
	return minPpe
}

func getPpeFromScore(score float32) float32 {
	if score < 1000 {
		return 0.03
	} else if score >= 1000 && score < 3000 {
		return 0.05
	} else if score >= 3000 && score < 5000 {
		return 0.07
	} else if score >= 5000 && score < 10000 {
		return 0.09
	} else if score >= 10000 && score < 20000 {
		return 0.15
	} else {
		return 0.20
	}
}
