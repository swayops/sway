package templates

import "github.com/hoisie/mustache"

func MustacheMust(s string) *mustache.Template {
	t, err := mustache.ParseString(s)
	if err != nil {
		panic(err)
	}
	return t
}
