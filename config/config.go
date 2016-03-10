package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"
	"time"
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

	DealTimeout    int32         `json:"dealTimeout"`    // In days
	StatsUpdate    time.Duration `json:"statsUpdate"`    // In hours
	StatsInterval  time.Duration `json:"statsInterval"`  // In seconds
	ExplorerUpdate time.Duration `json:"explorerUpdate"` // In hours

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
		User          string `json:"user"`
		Login         string `json:"login"`
		Token         string `json:"Token"`
		Ownership     string `json:"ownership"`
		ResetPassword string `json:"resetPassword"`
		Agency        string `json:"agency"`
		Group         string `json:"group"`
		Advertiser    string `json:"advertiser"`
		Campaign      string `json:"campaign"`
		Influencer    string `json:"influencer"`
	} `json:"bucket"`
}

func (c *Config) AllBuckets() []string {
	rv := reflect.ValueOf(c.Bucket)
	out := make([]string, 0, rv.NumField())
	for i := 0; i < cap(out); i++ {
		if v := rv.Field(i).String(); v != "" {
			out = append(out, v)
		}
	}
	return out
}
