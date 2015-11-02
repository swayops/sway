package config

import (
	"encoding/json"
	"log"
	"os"
)

func New(loc string) (*Config, error) {
	var c Config

	f, err := os.Open(loc)
	if err != nil {
		log.Println("Config error", err)
		return nil, err
	}

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		log.Println("Config error", err)
		return nil, err
	}

	return &c, nil
}

type Config struct {
	TwitterEndpoint string `json:"twitterEndpoint"`
	FbEndpoint      string `json:"fbEndpoint"`
	Instagram       struct {
		Endpoint string `json:"endpoint"`
		ClientId string `json:"clientId"`
	} `json:"instagram"`
}
