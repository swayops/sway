package main

import (
	"log"

	"github.com/swayops/sway/internal/config"
)

func main() {
	_, err := config.New("./config.sample.json")
	if err != nil {
		log.Fatal(err)
	}

}
