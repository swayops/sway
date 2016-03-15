package misc

import (
	"compress/flate"
	"compress/gzip"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const hour = int32(60 * 60)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrMissingId    = errors.New("missing id")

	nilTime time.Time
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

// CreateToken creates a token which consists of 2 bytes hash + 6 bytes timestamp(yyMMddhhmmss) + sz secure random data
// It will panic on the wrong size or random read errors since the program should NOT continue after that.
func CreateToken(sz int) []byte {
	if sz%8 != 0 {
		panic("size needs to be a multiple of 8")
	}
	buf := make([]byte, 8+sz)
	t := time.Now().UTC()
	buf[2], buf[3], buf[4] = byte(t.Year()-2000), byte(t.Month()), byte(t.Day())
	buf[5], buf[6], buf[7] = byte(t.Hour()), byte(t.Minute()), byte(t.Second())
	if _, err := rand.Read(buf[8:]); err != nil {
		panic(err)
	}
	binary.BigEndian.PutUint16(buf, Hash16(buf[2:]))
	return buf
}

func CheckToken(tok string) (time.Time, error) {
	buf := DecodeHex(tok)
	if len(buf) < 8 {
		return nilTime, ErrInvalidToken
	}
	if Hash16(buf[2:]) != binary.BigEndian.Uint16(buf) {
		return nilTime, ErrInvalidToken
	}
	t := time.Date(2000+int(buf[2]), time.Month(buf[3]), int(buf[4]), int(buf[5]), int(buf[6]), int(buf[7]), 0, time.UTC)
	return t, nil
}

func PseudoUUID() string {
	return hex.EncodeToString(CreateToken(8))
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

func Hash16(b []byte) uint16 {
	const prime = 374761393
	h := len(b)
	for _, c := range b {
		h = h*prime + int(c)
	}
	return uint16(h)
}

func DecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

func WithinLast(timestamp, hours int32) bool {
	// Is the timestamp within the last X hours?
	now := int32(time.Now().Unix())
	if timestamp >= (now - (hours * hour)) {
		return true
	}
	return false
}

// this will break "a x"@xxx.com, but seriously if someone is using a space in their name,
// maybe they shouldn't be allowed on the internet.
func TrimEmail(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
