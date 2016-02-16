package main

import (
	"log"
	"path/filepath"
	"strconv"
	"time"

	"os"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

func backupDatabases(cfg *config.Config, forced bool) (err error) {
	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	defer db.Close()

	dbPath := filepath.Join(cfg.DBPath, "backup", strconv.Itoa(time.Now().UTC().Hour()))
	if err = os.MkdirAll(dbPath, 0700); err != nil {
		return
	}

	dbFilePath := filepath.Join(dbPath, cfg.DBName+".db")
	if err = db.View(func(tx *bolt.Tx) error { return tx.CopyFile(dbFilePath, 0600) }); err != nil {
		return
	}

	if forced {
		log.Printf(`successfully backed up "%s.db" to %q.`, cfg.DBName, dbFilePath)
	}
	return
}

func backgroundBackup(cfg *config.Config) {
	dur, err := time.ParseDuration(cfg.BackupDuration)
	if err != nil || dur == 0 {
		return
	}
	for range time.Tick(dur) {
		if err := backupDatabases(cfg, false); err != nil {
			log.Printf("backgroundBackup error: %v", err)
		}
	}
}
