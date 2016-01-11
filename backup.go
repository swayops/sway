package main

import (
	"log"
	"path/filepath"
	"time"

	"os"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

func backupDatabases(cfg *config.Config) (err error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	defer db.Close()

	dbPath := filepath.Join(*backupPath, time.Now().UTC().Format(misc.StandardTimestamp))
	if err = os.MkdirAll(dbPath, 0700); err != nil {
		return
	}

	dbFilePath := filepath.Join(dbPath, cfg.DBName+".db")
	if err = db.View(func(tx *bolt.Tx) error { return tx.CopyFile(dbFilePath, 0600) }); err != nil {
		return
	}

	log.Printf(`successfully backed up "%s.db" to %q.`, cfg.DBName, dbFilePath)
	return
}
