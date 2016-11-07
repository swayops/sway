package common

import (
	"fmt"

	"encoding/base64"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

const format = "%s::%s"

func ShortenID(deal *Deal, tx *bolt.Tx, cfg *config.Config) string {
	id, err := misc.GetNextIndexBig(tx, cfg.Bucket.URL)
	if err != nil {
		return ""
	}
	idStr := base64.RawStdEncoding.EncodeToString(id.Bytes())

	if err := misc.PutBucketBytes(tx, cfg.Bucket.URL, idStr, []byte(fmt.Sprintf(format, deal.CampaignId, deal.Id))); err != nil {
		return ""
	}

	return idStr
}
