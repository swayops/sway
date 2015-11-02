package misc

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func Request(method, endpoint, reqData string, respData interface{}) error {
	client := &http.Client{}

	var r *http.Request
	if reqData == "" {
		r, _ = http.NewRequest(method, endpoint, nil)
	} else {
		r, _ = http.NewRequest(method, endpoint, strings.NewReader(reqData))
	}
	r.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(r)
	if err != nil {
		log.Println("Error when hitting:", endpoint, err)
		return err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		log.Println("Error when unmarshalling from:", endpoint, err)
		return err
	}
	return nil
}
