package main

import (
	"flag"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/server"
)

var (
	backup     = flag.Bool("backup", false, "backup the database(s) and exit")
	backupPath = flag.String("backupPath", "./backup", "backup path")
)

func main() {
	flag.Parse()
	cfg, err := config.New("config/config.json")
	if err != nil {
		log.Fatal(err)
	}
	switch {
	case *backup:
		err = backupDatabases(cfg)
	default:
		err = runServer(cfg)
	}
	if err != nil {
		log.Fatalln(err)
	}
}

func runServer(cfg *config.Config) error {
	r := gin.Default()
	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	srv, err := server.New(cfg, r)
	if err != nil {
		return err
	}

	// Listen and Serve
	if err = srv.Run(); err != nil {
		return err
	}
	return nil
}
