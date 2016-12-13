package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"

	"github.com/swayops/jlog"
	"github.com/swayops/sway/internal/geo"

	"github.com/missionMeteora/mandrill"
	"github.com/oschwald/maxminddb-golang"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
)

func New(loc string) (_ *Config, err error) {
	var c Config

	if err = loadJson(loc, &c); err != nil {
		log.Printf("error loading config: %v", err)
		return
	}

	if err = loadJson(loc+".user", &c); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("error loading userconfig: %v", err)
			return
		}
	}

	if c.TLS != nil && c.TLS.Port == "" {
		c.TLS.Port = "443"
	}

	c.GeoDB, err = geo.NewGeoDB(c.GeoLocation)
	if err != nil {
		log.Println("Config error", err)
		return nil, err
	}

	if c.Sandbox {
		c.ClickUrl = c.DashURL + "/cl/"
	} else {
		c.ClickUrl = c.HomeURL + "/cl/"
	}

	c.ec = mandrill.New(c.Mandrill.APIKey, c.Mandrill.SubAccount, c.Mandrill.FromEmail, c.Mandrill.FromName)
	c.replyEc = mandrill.New(c.Mandrill.APIKey, c.Mandrill.SubAccount, c.Mandrill.FromEmailReply, c.Mandrill.FromNameReply)

	jl, err := jlog.NewFromCfg(&jlog.Config{
		Path:    c.LogsPath,
		Loggers: []string{"ban", "deals", "stats", "charge", "email"},
	})
	if err != nil {
		log.Println("Config err!", err)
		return nil, err
	}
	c.Loggers = jl

	return &c, nil
}

func loadJson(fp string, out interface{}) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)

}

type Config struct {
	Host string `json:"host"`
	Port string `json:"port"`

	TLS *struct {
		Port string `json:"port"`
		Cert string `json:"certFile"`
		Key  string `json:"keyFile"`
	} `json:"tls"`

	DBPath        string `json:"dbPath"`
	DBName        string `json:"dbName"`
	BudgetDBName  string `json:"budgetDbName"`
	BudgetBuckets struct {
		Budget   string `json:"budget"`
		Balances string `json:"balance"`
	} `json:"budgetBuckets"`

	AuthDBName string `json:"authDbName"`

	Domain        string `json:"domain"`    // for cookies, important
	DashURL       string `json:"dashURL"`   // this is mainly used for internal directs
	InfAppURL     string `json:"infAppURL"` // this is mainly used for internal directs
	HomeURL       string `json:"homeURL"`   // this is mainly used for internal directs
	APIPath       string `json:"apiPath"`
	DashboardPath string `json:"dashboardPath"`
	InfAppPath    string `json:"infAppPath"`

	GeoLocation string            `json:"geoLoc"`
	GeoDB       *maxminddb.Reader `json:"geoDb"`

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
		Endpoint     string   `json:"endpoint"`
		AccessTokens []string `json:"accessTokens"`
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
		URL      string `json:"url"`
	} `json:"bucket"`

	SharpSpring struct {
		AccountID string `json:"id"`
		APIKey    string `json:"key"`
	} `json:"sharpSpring"`

	ec      *mandrill.Client
	replyEc *mandrill.Client

	Loggers *jlog.JLog

	JsonXlsxPath string `json:"jsonXlsxPath"`
	ImagesDir    string `json:"imagesDir"`
	ImageUrlPath string `json:"imageUrlPath"`

	LogsPath string `json:"logsPath"`

	ClickUrl string `json:"clickUrl"`
}

func (c *Config) AllBuckets(bk interface{}) []string {
	rv := reflect.ValueOf(bk)
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
