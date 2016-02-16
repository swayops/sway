package misc

import (
	"compress/flate"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
	"unsafe"
)

const StandardTimestamp = `20050102`

var (
	ErrMissingId = errors.New("missing id")
)

func HttpGetJson(c *http.Client, endpoint string, out interface{}) (err error) {
	var resp *http.Response
	if resp, err = c.Get(endpoint); err != nil {
		return
	}
	defer resp.Body.Close()

	switch resp.Header.Get("Content-Encoding") {
	case "":
		err = json.NewDecoder(resp.Body).Decode(out)
	case "gzip":
		var r *gzip.Reader
		if r, err = gzip.NewReader(resp.Body); err != nil {
			return
		}
		err = json.NewDecoder(r).Decode(out)
		r.Close()
	case "deflate":
		r := flate.NewReader(resp.Body)
		err = json.NewDecoder(r).Decode(out)
		r.Close()
	}

	return
}

// 9 bytes of unixnano and 7 random bytes
func PseudoUUID() string {
	now := time.Now().UnixNano()
	randPart := make([]byte, 7)
	if _, err := rand.Read(randPart); err != nil {
		copy(randPart, (*(*[8]byte)(unsafe.Pointer(&now)))[:7])
	}
	return strconv.FormatInt(now, 10)[1:] + hex.EncodeToString(randPart)
}

func DoesIntersect(opts []string, tg []string) bool {
	for _, o := range opts {
		for _, t := range tg {
			if t == o {
				return true
			}
		}
	}

	return false
}
