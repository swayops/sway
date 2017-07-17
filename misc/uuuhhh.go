package misc

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func BindJSON(c *gin.Context, v interface{}) error {
	body := c.Request.Body
	defer body.Close()
	return json.NewDecoder(body).Decode(v)
}

func WriteJSON(c *gin.Context, code int, v interface{}) error {
	c.Writer.Header().Set("Content-Type", gin.MIMEJSON)
	c.Status(code)
	return json.NewEncoder(c.Writer).Encode(v)
}
