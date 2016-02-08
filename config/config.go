package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
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
	Host string `json:"host"`
	Port string `json:"port"`

	DBPath string `json:"dbPath"`
	DBName string `json:"dbName"`

	Twitter struct {
		Endpoint     string `json:"endpoint"`
		Key          string `json:"key"`
		Secret       string `json:"secret"`
		AccessToken  string `json:"accessToken"`
		AccessSecret string `json:"accessSecret"`
	} `json:"twitter"`

	Tumblr struct {
		Endpoint     string `json:"endpoint"`
		Key          string `json:"key"`
		Secret       string `json:"secret"`
		AccessToken  string `json:"accessToken"`
		AccessSecret string `json:"accessSecret"`
	} `json:"tumblr"`

	YouTube struct {
		Endpoint string `json:"endpoint"`
		ClientId string `json:"clientId"`
	} `json:"youtube"`

	Instagram struct {
		Endpoint string `json:"endpoint"`
		ClientId string `json:"clientId"`
	} `json:"instagram"`

	Facebook struct {
		Endpoint string `json:"endpoint"`
		Id       string `json:"id"`
		Secret   string `json:"secret"`
	} `json:"facebook"`

	Bucket struct {
		User       string   `json:"user"`
		Agency     string   `json:"agency"`
		Group      string   `json:"group"`
		Advertiser string   `json:"advertiser"`
		Campaign   string   `json:"campaign"`
		Influencer string   `json:"influencer"`
		All        []string `json:"all"`
	} `json:"bucket"`
}
