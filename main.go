package main

import (
	"log"

	"github.com/swayops/sway/internal/config"
)

func main() {
	_, err := config.New("tests/config.sample.json")
	if err != nil {
		log.Fatal(err)
	}
}
