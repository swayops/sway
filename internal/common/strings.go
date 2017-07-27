package common

import (
	"strconv"
	"strings"

	"github.com/swayops/sway/misc"
)

func StringsIndexOf(hay []string, needle string) int {
	needle = strings.ToLower(needle)
	for i, s := range hay {
		if strings.ToLower(s) == needle {
			return i
		}
	}
	return -1
}

func IsInList(hay []string, needle string) bool {
	return StringsIndexOf(hay, needle) >= 0
}

// StringsRemove removes an item out of a slice, this *will* modify the original slice, YMMV
func StringsRemove(hay []string, needle string) []string {
	idx := StringsIndexOf(hay, needle)
	if ln := len(hay) - 1; idx > -1 {
		if ln == 0 { // len(hay) == 1
			return hay[:0]
		}
		hay[idx] = hay[ln]
		hay = hay[:ln]
	}
	return hay
}

func LowerSlice(s []string) []string {
	for i, v := range s {
		s[i] = strings.ToLower(strings.TrimSpace(v))
	}
	return s
}

func TrimEmails(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s))
	for i, v := range s {
		out[misc.TrimEmail(i)] = v
	}
	return out
}

func TrimWhitelist(s map[string]*Range) map[string]*Range {
	out := make(map[string]*Range, len(s))
	for i, v := range s {
		out[misc.TrimEmail(i)] = v
	}
	return out
}

func SliceMap(s map[string]bool) []string {
	out := make([]string, 0, len(s))
	for k, _ := range s {
		out = append(out, k)
	}
	return out
}

func SliceWhitelist(s map[string]*Range) []string {
	out := make([]string, 0, len(s))
	for k, _ := range s {
		out = append(out, k)
	}
	return out
}

func Map(s []string) map[string]bool {
	out := make(map[string]bool)
	for _, k := range s {
		out[k] = true
	}
	return out
}

var wordBoundaries = [255]bool{
	' ': true,
	'.': true,
	',': true,
	':': true,
	'@': true,
	'+': true,
}

func IsExactMatch(haystack, needle string) bool {
	haystack, needle = strings.ToLower(haystack), strings.ToLower(needle)
	if idx := strings.Index(haystack, needle); idx > -1 {
		if idx != 0 && !wordBoundaries[haystack[idx-1]] {
			return false
		}

		if end := idx + len(needle); end != len(haystack) && !wordBoundaries[haystack[end]] {
			return false
		}

		return true
	}
	return false
}

func Commanize(v int64) string {
	sign := ""

	if v < 0 {
		sign = "-"
		v = 0 - v
	}

	parts := []string{"", "", "", "", "", "", ""}
	j := len(parts) - 1

	for v > 999 {
		parts[j] = strconv.FormatInt(v%1000, 10)
		switch len(parts[j]) {
		case 2:
			parts[j] = "0" + parts[j]
		case 1:
			parts[j] = "00" + parts[j]
		}
		v = v / 1000
		j--
	}
	parts[j] = strconv.Itoa(int(v))
	return sign + strings.Join(parts[j:], ",")
}
