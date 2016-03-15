package common

var CATEGORIES = map[string]struct{}{
	"vlogger": struct{}{},
}

func GetCategories() (out []string) {
	for k, _ := range CATEGORIES {
		out = append(out, k)
	}
	return out
}
