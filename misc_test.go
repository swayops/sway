package main

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
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/swayops/resty"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/server"
)

type M map[string]interface{}

var (
	printResp = flag.Bool("pr", false, "print responses")
	keepTmp   = flag.Bool("k", false, "keep tmp dir")

	insecureTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	ts   *httptest.Server
	rstP = sync.Pool{
		New: func() interface{} {
			rst := resty.NewClient(ts.URL)
			rst.HTTPClient.Transport = insecureTransport
			return rst
		},
	}
)

func TestMain(m *testing.M) {
	log.SetFlags(log.Lshortfile | log.Ltime)

	var code int = 1
	defer func() { os.Exit(code) }()

	cfg, err := config.New("config/config.json")
	panicIf(err)

	cfg.Sandbox = true // always set it to true just in case

	cfg.DBPath, err = ioutil.TempDir("", "sway-srv")
	panicIf(err)

	if *keepTmp {
		log.Println("tmp dir:", cfg.DBPath)
	} else {
		defer os.RemoveAll(cfg.DBPath) // clean up
	}

	r := gin.Default()
	srv, err := server.New(cfg, r)
	panicIf(err)

	ts = httptest.NewTLSServer(r)
	defer ts.CloseClientConnections()
	defer ts.Close()

	code = m.Run()

	_ = srv
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
		log.Panicln(err)
	}
}

func getClient() *resty.Client {
	rst := rstP.Get().(*resty.Client)
	return rst
}

func putClient(c *resty.Client) {
	c.Reset()
	rstP.Put(c)
}

func R(status int, v interface{}) (r resty.Reply) {
	r.Status = status
	switch v := v.(type) {
	case []byte:
		r.Value = v
	case string:
		r.Value = []byte(v)
	case nil:
	default:
		r.Value, r.Err = json.Marshal(v)
	}
	return
}

type testReq struct {
	method, path string
	data         interface{}
	expected     resty.Reply
}

func (tr *testReq) String() string {
	return tr.method + " " + tr.path
}

func (tr *testReq) run(t *testing.T, c *resty.Client) {
	r := c.Do(tr.method, tr.path, tr.data, nil)
	if *printResp {
		t.Logf("%s: %s", tr.String(), r.Value)
	}
	ex := &tr.expected
	switch {
	case ex.Err != nil:
		t.Fatalf("%s: %v, %s", tr.String(), ex.Err, r.Value)
	case r.Err != nil:
		t.Fatalf("%s: error: %v, status: %d, resp: %s", tr.String(), r.Err, r.Status, r.Value)
	case ex.Status != 0 && r.Status != 200, r.Status != ex.Status:
		t.Fatalf("%s: wanted %d, got %d: %s", tr.String(), ex.Status, r.Status, r.Value)
	case ex.Value != nil:
		if err := compareRes(r.Value, ex.Value); err != nil {
			t.Fatalf("%s: %v", tr.String(), err)
		}
	}
}
