package tests

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/missionMeteora/iodb"
)

const cachePath = `./cachedb/`

var db *iodb.DB

func TestMain(m *testing.M) {
	flag.Parse()
	var err error
	if db, err = iodb.New(cachePath, nil); err != nil {
		log.Fatalln(err)
	}
	log.Printf("created cachedb at %s", cachePath)
	var ret int
	defer func() { os.Exit(ret) }()
	ret = m.Run()
	db.Close()
	os.RemoveAll(cachePath)
}
