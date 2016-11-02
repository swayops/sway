package misc

import (
	"net/http"
	"time"
)

func SetCookie(w http.ResponseWriter, domain, name, value string, secure bool, dur time.Duration) {
	cookie := &http.Cookie{
		Path:     "/",
		Domain:   domain,
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   secure,
	}
	if dur > 0 {
		cookie.Expires = time.Now().Add(dur)
	} else {
		cookie.MaxAge = -1
	}

	http.SetCookie(w, cookie)
}

func RefreshCookie(w http.ResponseWriter, r *http.Request, domain, name string, dur time.Duration) {
	c, err := r.Cookie(name)
	if err != nil {
		return
	}

	c.Path, c.Expires = "/", time.Now().Add(dur)
	c.Domain = domain

	http.SetCookie(w, c)
}

func GetCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}

func DeleteCookie(w http.ResponseWriter, domain, name string, secure bool) {
	SetCookie(w, domain, name, "deleted", secure, -1)
}
