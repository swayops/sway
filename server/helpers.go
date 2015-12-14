package server

import "github.com/gin-gonic/gin"

func createRoutes(r *gin.Engine, srv *Server, endpoint string, get, post, del func(*Server) gin.HandlerFunc) {
	r.GET(endpoint+"/:id", get(srv))
	r.POST(endpoint, post(srv))
	r.DELETE(endpoint+"/:id", del(srv))
}
