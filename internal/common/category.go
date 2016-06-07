package common

var CATEGORIES = map[string]struct{}{
	"vlogger": struct{}{},
}

func GetCategories() []string {
	out := make([]string, 0, len(CATEGORIES))
	for k, _ := range CATEGORIES {
		out = append(out, k)
	}
	return out
}
