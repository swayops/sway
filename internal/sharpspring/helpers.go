package sharpspring

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

var (
	debug = false

	emptyObj = []byte("[]")
	null     = []byte("null")
)

func parseResponse(rd io.ReadCloser) error {
	defer rd.Close()

	var j json.RawMessage
	if err := json.NewDecoder(rd).Decode(&j); err != nil {
		return err
	}

	// because fuck php's json encoder, that's why
	j = bytes.Replace(j, emptyObj, null, -1)

	if debug {
		log.Printf("%s", j)
	}

	var r ssResponse
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		return err
	}

	var errs []string
	for m, resps := range r.Result {
		for _, resp := range resps {
			if resp.Success {
				continue
			}
			if resp.Error == nil {
				return fmt.Errorf("unknown error: %s", j)
			}
			if resp.Error.Data == nil {
				errs = append(errs, "(method: "+m+"): "+resp.Error.Message)
			} else {
				for _, p := range resp.Error.Data.Params {
					errs = append(errs, "(method: "+m+") "+p.Param+": "+p.Message)
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

type ssResponse struct {
	Result map[string][]struct {
		Success bool `json:"success"`
		Error   *struct {
			Message string `json:"message"`
			Data    *struct {
				Params []*struct {
					Param   string `json:"param"`
					Message string `json:"message"`
				} `json:"params"`
			} `json:"data"`
		} `json:"error"`
	}
}
