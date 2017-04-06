package server

import (
	"archive/tar"
	"compress/gzip"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
)

func dumpDatabases(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			gzw = gzip.NewWriter(c.Writer)
			tw  = tar.NewWriter(gzw)
			ts  = "dbs-" + time.Now().UTC().Format(`2006-01-02-15-04`)
		)

		defer func() {
			// should never ever ever happen, but if it does, don't leak our crap
			if v := recover(); v != nil {
				log.Printf("that's not good: %T %v", v, v)
			}

			tw.Close()
			gzw.Close()
		}()

		c.Header("Content-Type", "application/x-gzip")
		c.Header("Content-Disposition", `attachment; filename="`+ts+`.tar.gz"`)

		s.db.View(func(tx *bolt.Tx) (err error) {
			hdr := &tar.Header{
				Name: "data/sway.db",
				Mode: 0600,
				Size: tx.Size(),
			}

			if err = tw.WriteHeader(hdr); err != nil {
				log.Printf("error dumping sway.db: %v", err)
				return
			}
			if _, err = tx.WriteTo(tw); err != nil {
				log.Printf("error dumping sway.db: %v", err)
				return
			}

			return
		})

		s.budgetDb.View(func(tx *bolt.Tx) (err error) {
			hdr := &tar.Header{
				Name: "data/budget.db",
				Mode: 0600,
				Size: tx.Size(),
			}

			if err = tw.WriteHeader(hdr); err != nil {
				log.Printf("error dumping budget.db: %v", err)
				return
			}
			if _, err = tx.WriteTo(tw); err != nil {
				log.Printf("error dumping budget.db: %v", err)
				return
			}

			return
		})
	}
}
