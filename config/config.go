package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/missionMeteora/mandrill"
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
	c.ec = mandrill.New(c.Mandrill.APIKey, c.Mandrill.SubAccount, c.Mandrill.FromEmail, c.Mandrill.FromName)
	return &c, nil
}

type Config struct {
	Host string `json:"host"`
	Port string `json:"port"`

	DBPath       string `json:"dbPath"`
	DBName       string `json:"dbName"`
	BudgetDBName string `json:"budgetDbName"`
	AuthDBName   string `json:"authDbName"`
	BudgetBucket string `json:"budgetBucket"`

	ServerURL string `json:"serverURL"` // this is mainly used for internal directs
	APIPath   string `json:"apiPath"`

	Sandbox bool `json:"sandbox"`

	DealTimeout   int32         `json:"dealTimeout"`   // In days
	EngineUpdate  time.Duration `json:"engineUpdate"`  // In hours
	StatsInterval time.Duration `json:"statsInterval"` // In seconds
	InfluencerTTL int32         `json:"influencerTtl"` // In hours

	Mandrill struct {
		APIKey     string `json:"apiKey"`
		SubAccount string `json:"subAccount"`
		FromEmail  string `json:"fromEmail"`
		FromName   string `json:"fromName"`
	} `json:"mandrill"`

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
		AdAgency     string `json:"adAgency"`
		TalentAgency string `json:"talentAgency"`
		Advertiser   string `json:"advertiser"`
		Campaign     string `json:"campaign"`
		Influencer   string `json:"influencer"`
	} `json:"bucket"`

	AuthBucket struct {
		User      string `json:"user"`
		Login     string `json:"login"`
		Token     string `json:"Token"`
		Ownership string `json:"ownership"`
	} `json:"authBucket"`

	ec *mandrill.Client
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

func (c *Config) AllAuthBuckets() []string {
	rv := reflect.ValueOf(c.AuthBucket)
	out := make([]string, 0, rv.NumField())
	for i := 0; i < cap(out); i++ {
		if v := rv.Field(i).String(); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (c *Config) MailClient() *mandrill.Client {
	return c.ec
}
