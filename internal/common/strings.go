package common

func StringsIndexOf(hay []string, needle string) int {
	for i, s := range hay {
		if s == needle {
			return i
		}
	}
	return -1
}

// StringsRemove removes an item out of a slice, this *will* modify the original slice, YMMV
func StringsRemove(hay []string, needle string) []string {
	idx := StringsIndexOf(hay, needle)
	if idx > -1 {
		copy(hay[idx:], hay[idx+1:])
		ln := len(hay) - 1
		hay, hay[ln] = hay[:ln], ""
	}
	return hay
}
