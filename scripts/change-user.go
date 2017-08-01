// +build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/internal/auth"

	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

func main() {
	var (
		uid    = flag.String("uid", "", "user id")
		nemail = flag.String("e", "", "new email")
		npass  = flag.String("p", "", "new password")
	)

	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile)

	if *uid == "" || *nemail == "" || *npass == "" {
		log.Fatal("invalid: %s %s %s", *uid, *nemail, *npass)
	}

	cfg, err := config.New("config/config.json")
	if err != nil {
		log.Fatal(err)
	}

	db := misc.OpenDB(cfg.DBPath, cfg.DBName)
	defer db.Close()
	a := auth.New(db, cfg)

	if err = db.Update(func(tx *bolt.Tx) error {
		var (
			u        = a.GetUserTx(tx, *uid)
			lB       = misc.GetBucket(tx, cfg.Bucket.Login)
			l        auth.Login
			newEmail = misc.TrimEmail(*nemail)
		)

		if u == nil {
			return fmt.Errorf("couldn't find uid: %s", *uid)
		}

		if misc.GetTxJson(tx, cfg.Bucket.Login, u.Email, &l); err != nil {
			return fmt.Errorf("error getting login: %v", err)
		}

		//log.Printf("%#+v\n%#+v", l, u)

		l.Password, _ = auth.HashPassword(*npass)
		l.UserID = u.ID

		if err := misc.PutTxJson(tx, cfg.Bucket.Login, newEmail, &l); err != nil {
			return fmt.Errorf("error putting login: %v", err)
		}

		if u.Email != newEmail {
			lB.Delete([]byte(u.Email))
			u.Email = newEmail
			return misc.PutTxJson(tx, cfg.Bucket.User, u.ID, u)
		}
		return nil
	}); err != nil {
		log.Println(err)
		return
	}
	log.Printf("successfully changed %s's email to %s", *uid, *nemail)
}
