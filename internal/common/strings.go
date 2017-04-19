package common

import (
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

func SliceMap(s map[string]bool) []string {
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

func IsExactMatch(haystack, needle string) bool {
	haystack = strings.ToLower(haystack)
	needle = strings.ToLower(needle)

	if idx := strings.Index(haystack, needle); idx >= 0 {
		var before, after bool
		if idx-1 < 0 || string(haystack[idx-1]) == " " {
			before = true
		}

		if idx+len(needle) >= len(haystack) || string(haystack[idx+len(needle)]) == " " {
			after = true
		}

		if before && after {
			return true
		}
	}
	return false
}
