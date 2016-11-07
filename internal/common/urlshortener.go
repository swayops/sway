package common

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

var format = "%s::%s"

func ShortenID(deal *Deal, tx *bolt.Tx, cfg *config.Config) string {
	id, err := misc.GetNextIndex(tx, cfg.Bucket.URL)
	if err != nil {
		return ""
	}
	if err := misc.PutBucketBytes(tx, cfg.Bucket.URL, id, []byte(fmt.Sprintf(format, deal.CampaignId, deal.Id))); err != nil {
		return ""
	}

	return id
}
