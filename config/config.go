package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"

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
	c.replyEc = mandrill.New(c.Mandrill.APIKey, c.Mandrill.SubAccount, c.Mandrill.FromEmailReply, c.Mandrill.FromNameReply)
	return &c, nil
}

type Config struct {
	Host string `json:"host"`
	Port string `json:"port"`

	DBPath          string `json:"dbPath"`
	DBName          string `json:"dbName"`
	BudgetDBName    string `json:"budgetDbName"`
	BudgetBucket    string `json:"budgetBucket"`
	ReportingDBName string `json:"reportingDbName"`
	ReportingBucket string `json:"reportingBucket"`
	AuthDBName      string `json:"authDbName"`

	ServerURL string `json:"serverURL"` // this is mainly used for internal directs
	APIPath   string `json:"apiPath"`

	Sandbox bool `json:"sandbox"`

	Mandrill struct {
		APIKey         string `json:"apiKey"`
		SubAccount     string `json:"subAccount"`
		FromEmail      string `json:"fromEmail"`
		FromName       string `json:"fromName"`
		FromEmailReply string `json:"fromEmailReply"`
		FromNameReply  string `json:"fromNameReply"`
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
		Endpoint    string `json:"endpoint"`
		AccessToken string `json:"accessToken"`
	} `json:"instagram"`

	Facebook struct {
		Endpoint string `json:"endpoint"`
		Id       string `json:"id"`
		Secret   string `json:"secret"`
	} `json:"facebook"`

	Bucket struct {
		User     string `json:"user"`
		Login    string `json:"login"`
		Token    string `json:"token"`
		Campaign string `json:"campaign"`
		Scrap    string `json:"scrap"`
	} `json:"bucket"`

	ec      *mandrill.Client
	replyEc *mandrill.Client

	JsonXlsxPath string `json:"jsonXlsxPath"`
	ImagesDir    string `json:"imagesDir"`
	ImageUrlPath string `json:"imageUrlPath"`
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

func (c *Config) MailClient() *mandrill.Client {
	return c.ec
}

func (c *Config) ReplyMailClient() *mandrill.Client {
	return c.replyEc
}
