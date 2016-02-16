package misc

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

func StatusOK(id string) gin.H {
	if len(id) == 0 {
		return gin.H{"status": "success"}
	}
	return gin.H{"status": "success", "id": id}
}

func StatusErr(msg string) gin.H {
	return gin.H{"status": "error", "msg": msg}
}
