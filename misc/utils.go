package misc

import (
	"compress/flate"
	"compress/gzip"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

const hour = int32(60 * 60)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrMissingId    = errors.New("missing id")

	nilTime time.Time

	rnd struct {
		sync.Mutex
		*rand.Rand
	}
)

func init() {
	var seed [8]byte
	if n, err := crand.Read(seed[:]); n != 8 || err != nil {
		panic("couldn't generate a crypto rand seed")
	}
	seedn := binary.BigEndian.Uint64(seed[:])
	rnd.Rand = rand.New(rand.NewSource(int64(seedn)))
}

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
	rnd.Lock()
	_, err := rnd.Read(buf[8:])
	rnd.Unlock()
	if err != nil {
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

func Contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
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
	now := int32(time.Now().Unix())

	// If the timestamp is from the future.. bail!
	if timestamp > now {
		return false
	}

	// Is the timestamp within the last X hours?
	if timestamp >= (now - (hours * hour)) {
		return true
	}
	return false
}

func WithinHours(timestamp, min, max int32) bool {
	// Is the timestamp within the next min to max hours?
	now := int32(time.Now().Unix())
	minTs := now + (min * hour)
	maxTs := now + (max * hour)

	if timestamp >= minTs && timestamp < maxTs {
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

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func TruncateFloat(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func Random(min, max int) int32 {
	return int32(rand.Intn(max-min) + min)
}

func SanitizeHashes(str []string) []string {
	// Removes # from string
	out := make([]string, 0, len(str))
	for _, s := range str {
		out = append(out, SanitizeHash(s))
	}
	return out
}

func SanitizeHash(str string) string {
	// Removes starting #
	if strings.HasPrefix(str, "#") {
		str = str[1:]
	}
	return strings.ToLower(str)
}
