package config

import (
	"encoding/json"
	"os"
)

func New(loc string) (*Config, error) {
	var c Config

	f, err := os.Open(loc)
	if err != nil {
		return nil, err
	}

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

type Config struct {
	TwitterEndpoint string `json:"twitterEndpoint"`
	FbEndpoint      string `json:"fbEndpoint"`
	InstaEndpoint   string `json:"instaEndpoint"`
}
