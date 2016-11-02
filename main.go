package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/swayops/closer"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/server"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile)

	cfg, err := config.New("config/config.json")
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.Sandbox {
		gin.SetMode(gin.ReleaseMode)
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

	defer func() {
		recover() // ignore the panic
		closer.Defer(srv.Close)()
	}()

	// Listen and Serve
	if err = srv.Run(); err != nil {
		// using panic rather than fatal because fatal would terminal the program
		// and it would never call our closer
		log.Panicf("Failed to listen: %v", err)
	}

}
