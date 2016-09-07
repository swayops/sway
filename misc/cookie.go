package misc

import (
	"net/http"
	"time"
)

func SetCookie(w http.ResponseWriter, name, value string, dur time.Duration) {
	cookie := &http.Cookie{
		Path:     "/",
		Name:     name,
		Value:    value,
		HttpOnly: true,
		//Secure:   true,
	}
	if dur > 0 {
		cookie.Expires = time.Now().Add(dur)
	} else {
		cookie.MaxAge = -1
	}
	http.SetCookie(w, cookie)
}

func RefreshCookie(w http.ResponseWriter, r *http.Request, name string, dur time.Duration) {
	c, err := r.Cookie(name)
	if err != nil {
		return
	}
	c.Path, c.Expires = "/", time.Now().Add(dur)
	http.SetCookie(w, c)
}

func GetCookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err != nil {
		return ""
	} else {
		return c.Value
	}
}

func DeleteCookie(w http.ResponseWriter, name string) {
	SetCookie(w, name, "deleted", -1)
}