package common

var CATEGORIES = map[string]struct{}{
	"arts & entertainment":       struct{}{},
	"automotive":                 struct{}{},
	"business":                   struct{}{},
	"career advice":              struct{}{},
	"family & parenting":         struct{}{},
	"health & fitness":           struct{}{},
	"food & drink":               struct{}{},
	"hobbies & interests":        struct{}{},
	"home & garden":              struct{}{},
	"law, gov't & politics":      struct{}{},
	"news":                       struct{}{},
	"personal finance":           struct{}{},
	"dating/ weddings/ marriage": struct{}{},
	"science":                    struct{}{},
	"pets":                       struct{}{},
	"sports":                     struct{}{},
	"style & fashion":            struct{}{},
	"technology & computing":     struct{}{},
	"travel":                     struct{}{},
	"real estate":                struct{}{},
	"shopping":                   struct{}{},
	"religion & spirituality":    struct{}{},
}

func GetCategories() []string {
	out := make([]string, 0, len(CATEGORIES))
	for k, _ := range CATEGORIES {
		out = append(out, k)
	}
	return out
}
