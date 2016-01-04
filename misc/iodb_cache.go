package misc

import (
	"bytes"
	"io"
	"time"

	"github.com/missionMeteora/iodb"
)

const DefaultCacheDuration = 1 * time.Hour

type Platform string

const (
	PlatformTwitter   Platform = "twitter"
	PlatformInstagram          = "instagram"
	PlatformFacebook           = "facebook"
	PlatformYoutube            = "youtube"
	PlatformTumblr             = "tumblr"
)

func GetPlatformCache(db *iodb.DB, platform Platform, pid string) (rc io.ReadCloser) {
	b := db.Bucket(string(platform))
	if b == nil {
		return nil
	}
	rc, _ = b.Get(pid)
	return
}

func PutPlatformCache(db *iodb.DB, platform Platform, pid string, data []byte, dur time.Duration) error {
	b, err := db.CreateBucket(string(platform))
	if err != nil {
		return nil
	}
	if dur > 0 {
		return b.PutTimed(pid, bytes.NewReader(data), dur)
	}
	return b.Put(pid, bytes.NewReader(data))
}
