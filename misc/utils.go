package misc

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrMissingId = errors.New("missing id")
)

func HttpGetJson(c *http.Client, endpoint string, out interface{}) (err error) {
	var resp *http.Response
	if resp, err = c.Get(endpoint); err != nil {
		return
	}
	defer resp.Body.Close()

	r := resp.Body
	if resp.Header.Get("Content-Encoding") != "" {
		var gr *gzip.Reader
		if gr, err = gzip.NewReader(resp.Body); err != nil {
			return
		}
		defer gr.Close()
		r = gr
	}

	return json.NewDecoder(r).Decode(out)
}
