package misc

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	client    http.Client
	ErrStatus = errors.New("non-200 status code")
)

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

func Ping(endpoint string) error {
	endpoint = strings.Replace(endpoint, " ", "%20", -1)

	r, _ := http.NewRequest("GET", endpoint, nil)

	resp, err := client.Do(r)
	if err != nil {
		log.Println("Error when hitting:", endpoint, err)
		return err
	}

	resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrStatus
	}

	return nil
}

func StatusOK(id string) gin.H {
	if len(id) == 0 {
		return gin.H{"status": "success"}
	}
	return gin.H{"status": "success", "id": id, "code": 200}
}

func StatusErr(msg string) gin.H {
	return gin.H{"status": "error", "msg": msg, "code": 400}
}

func AbortWithErr(c *gin.Context, code int, err error) {
	m := StatusErr(err.Error())
	m["code"] = code
	c.JSON(code, m)
	c.Abort()
}
