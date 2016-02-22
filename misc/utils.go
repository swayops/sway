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
	"time"
)

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

// CreateToken creates a token which consists of 2 bytes hash + 6 bytes timestamp(yyyyMMddhhmm) + sz secure random data
// It will panic on the wrong size or random read errors since the program should NOT continue after that.
func CreateToken(sz int) []byte {
	if sz%8 != 0 {
		panic("size needs to be a multiple of 8")
	}
	buf := make([]byte, 8+sz)
	t := time.Now().UTC()
	binary.BigEndian.PutUint16(buf[2:], uint16(t.Year()))
	buf[4], buf[5], buf[6], buf[7] = byte(t.Month()), byte(t.Day()), byte(t.Hour()), byte(t.Minute())
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
	t := time.Date(int(binary.BigEndian.Uint16(buf[2:])), time.Month(buf[4]), int(buf[5]), int(buf[6]), int(buf[7]), 0, 0, time.UTC)
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
