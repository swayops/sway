package auth

import (
	"net/http"
	"time"
)

func setCookie(w http.ResponseWriter, name, value string, dur time.Duration) {
	cookie := &http.Cookie{
		Path:    "/",
		Name:    name,
		Value:   value,
		Expires: time.Now().Add(dur),
	}
	http.SetCookie(w, cookie)
}

func refreshCookie(w http.ResponseWriter, r *http.Request, name string, dur time.Duration) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return
	}
	cookie.Expires = time.Now().Add(dur)
	cookie.Path = "/"
	http.SetCookie(w, cookie)
}

func getCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}
