package common

import "strings"

func StringsIndexOf(hay []string, needle string) int {
	for i, s := range hay {
		if s == needle {
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
