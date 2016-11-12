package server

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go"
	"github.com/swayops/resty"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/internal/auth"
	"github.com/swayops/sway/platforms/swipe"
)

type M map[string]interface{}

var (
	printResp  = flag.Bool("pr", os.Getenv("PR") != "", "print responses")
	genData    = flag.Bool("gen", os.Getenv("gen") != "", "leave the test data")
	creditCard = &swipe.CC{
		FirstName:  "John",
		LastName:   "Smith",
		Address:    "8 Saint Elias",
		City:       "Trabuco Canyon",
		State:      "CA",
		Country:    "US",
		Zip:        "92679",
		CardNumber: "4242424242424242",
		CVC:        "123",
		ExpMonth:   "06",
		ExpYear:    "20",
	}
	newCreditCard = &swipe.CC{
		FirstName:  "New",
		LastName:   "CC",
		Address:    "8 Saint Elias",
		City:       "Trabuco Canyon",
		State:      "CA",
		Country:    "US",
		Zip:        "92679",
		CardNumber: "4242424242424242",
		CVC:        "321",
		ExpMonth:   "06",
		ExpYear:    "20",
	}

	cfg *config.Config

	insecureTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	ts   *httptest.Server
	rstP = sync.Pool{
		New: func() interface{} {
			rst := resty.NewClient(ts.URL + "/api/v1/")
			rst.HTTPClient.Transport = insecureTransport
			return rst
		},
	}

	srv *Server
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime)
	flag.Parse()

	panicIf(os.Chdir("..")) // this is for the relative paths in config, like imageDir and geo.

	resty.LogRequests = *printResp
}

func TestMain(m *testing.M) {
	var (
		code int = 1
		err  error
	)
	defer func() { os.Exit(code) }()

	cfg, err = config.New("./config/config.json")
	panicIf(err)

	stripe.Key = "sk_test_t6NYedi21SglECi1HwEvSMb8"
	cfg.Sandbox = true // always set it to true just in case

	if !*genData {
		cfg.DBPath, err = ioutil.TempDir("", "sway-srv")
		panicIf(err)

		defer os.RemoveAll(cfg.DBPath) // clean up
	}

	// disable all the gin spam
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	srv, err = New(cfg, r)
	panicIf(err)

	ts = httptest.NewTLSServer(r)
	ts.URL = strings.Replace(ts.URL, "127.0.0.1", "dash.swayops.fake", 1)
	defer ts.Close()

	code = m.Run()
}

func compareRes(a, b []byte) error {
	var am, bm M
	if err := json.Unmarshal(a, &am); err != nil {
		return fmt.Errorf("%s: %v", a, err)
	}
	if err := json.Unmarshal(b, &bm); err != nil {
		return fmt.Errorf("%s: %v", b, err)
	}
	if !reflect.DeepEqual(am, bm) {
		return fmt.Errorf("%s != %s", a, b)
	}
	return nil
}

func panicIf(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func getClient() *resty.Client { return rstP.Get().(*resty.Client) }

func putClient(c *resty.Client) {
	c.Reset()
	rstP.Put(c)
}

type signupUser struct {
	*auth.User
	Password  string `json:"pass"`
	Password2 string `json:"pass2"`
	ExpID     string `json:"-"`
}

const defaultPass = "12345678"

var counter int = 3 // 3 is the highest built in user (TalentAgency)

func getSignupUser() *signupUser {
	counter++
	id := strconv.Itoa(counter)
	name := "John " + id

	return &signupUser{
		&auth.User{
			Name:  name,
			Email: name + "@a.b",
		},
		defaultPass,
		defaultPass,
		id,
	}
}

func getSignupUserWithName(name string) *signupUser {
	counter++
	id := strconv.Itoa(counter)
	return &signupUser{
		&auth.User{
			Name:  name,
			Email: name + "@a.b",
		},
		defaultPass,
		defaultPass,
		id,
	}
}

func getSignupUserWithEmail(email string) *signupUser {
	counter++
	id := strconv.Itoa(counter)
	name := "John " + id
	return &signupUser{
		&auth.User{
			Name:  name,
			Email: email,
		},
		defaultPass,
		defaultPass,
		id,
	}
}
