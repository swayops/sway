package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"strings"
	"time"

	"io/ioutil"

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

	log.Println("Stripe:", cfg.Stripe.Key)
	log.Println("Lob:", cfg.Lob.Key, cfg.Lob.Addr, cfg.Lob.BankAcct)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger(cfg.Sandbox, "/static", "/favicon.ico", "/api/v1/getIncompleteInfluencers"))

	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	srv, err := server.New(cfg, r)
	if err != nil {
		log.Fatal(err)
	}

	defer closer.Defer(srv.Close)()

	// Listen and Serve
	if err = srv.Run(); err != nil {
		// using panic rather than fatal because fatal would terminal the program
		// and it would never call our closer
		log.Panicf("Failed to listen: %v", err)
	}

}

func ginLogger(sandbox bool, prefixesToSkip ...string) gin.HandlerFunc {
	// shamelessly copied from gin.Logger
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, pre := range prefixesToSkip {
			if strings.HasPrefix(path, pre) {
				return
			}
		}
		start := time.Now()

		if sandbox {
			switch m := c.Request.Method; m {
			case "POST", "PUT", "DELETE":
				var buf bytes.Buffer
				io.Copy(&buf, c.Request.Body)
				c.Request.Body.Close()
				c.Request.Body = ioutil.NopCloser(&buf)
				j, _ := json.Marshal(c.Request.Header)
				if ln := buf.Len(); ln > 0 {
					switch buf.Bytes()[0] {
					case '[', '{', 'n': // [], {} and nullable
						log.Printf("%s: %s\n\tHeaders: %s\n\tRequest (%d): %s", m, path, j, ln, buf.String())
					default:
						log.Printf("%s: %s\n\t\n\tHeaders: %s\n\tRequest (%d): <binary>", m, path, j, ln)
					}
				}
			}
		}

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		log.Printf("[%s] [%d] %s %s [%s]", clientIP, statusCode, method, path, latency)
	}
}
