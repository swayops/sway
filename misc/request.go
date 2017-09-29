package misc

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	ErrStatus = errors.New("non-200 status code")
)

var (
	client = http.Client{
		Timeout: 5 * time.Second,
	}
)

func Request(method, endpoint, reqData string, respData interface{}) (err error) {
	endpoint = strings.Replace(endpoint, " ", "%20", -1)

	var (
		r    *http.Request
		resp *http.Response
	)

	if reqData == "" {
		r, err = http.NewRequest(method, endpoint, nil)
	} else {
		r, err = http.NewRequest(method, endpoint, strings.NewReader(reqData))
	}
	if err != nil {
		return
	}

	r.Header.Add("Content-Type", "application/json")

	if resp, err = client.Do(r); err != nil {
		log.Println("Error when hitting:", endpoint, err)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&respData)
	resp.Body.Close()
	if err != nil {
		log.Println("Error when unmarshalling from:", endpoint, err)
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

func StatusOKExtended(id string, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	data["status"], data["code"] = "success", 200
	if id != "" {
		data["id"] = id
	}
	return data
}

func StatusErr(msg string) gin.H {
	return gin.H{"status": "error", "msg": msg, "code": 400}
}

func AbortWithErr(c *gin.Context, code int, err error) {
	m := StatusErr(err.Error())
	m["code"] = code
	WriteJSON(c, code, m)
	c.Abort()
}
