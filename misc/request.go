package misc

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var client http.Client

func Request(method, endpoint, reqData string, respData interface{}) error {
	endpoint = strings.Replace(endpoint, " ", "%20", -1)

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

	err = json.NewDecoder(resp.Body).Decode(&respData)
	resp.Body.Close()
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

func AbortWithErr(c *gin.Context, code int, err error) {
	m := StatusErr(err.Error())
	m["code"] = code
	c.JSON(code, m)
	c.Abort()
}
