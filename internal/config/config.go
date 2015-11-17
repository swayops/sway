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
	Twitter struct {
		Endpoint string `json:"endpoint"`
		Key      string `json:"key"`
		Secret   string `json:"secret"`
	} `json:"twitter"`

	FbEndpoint string `json:"fbEndpoint"`

	YouTube struct {
		Endpoint string `json:"endpoint"`
		ClientId string `json:"clientId"`
	} `json:"youtube"`

	Instagram struct {
		Endpoint string `json:"endpoint"`
		ClientId string `json:"clientId"`
	} `json:"instagram"`
}
