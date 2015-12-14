package main

import (
	"log"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/server"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	cfg, err := config.New("config/config.json")
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	srv, err := server.New(cfg, r)
	if err != nil {
		log.Fatal(err)
	}

	// Listen and Serve
	srv.Run()
}
