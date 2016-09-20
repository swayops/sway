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
	out := make(map[string]bool)
	for i, v := range s {
		out[misc.TrimEmail(i)] = v
	}
	return out
}

func Slice(s map[string]bool) []string {
	out := []string{}
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
