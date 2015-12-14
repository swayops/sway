package misc

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/boltdb/bolt"
)

func OpenDB(path string, name string) *bolt.DB {
	db, err := bolt.Open(path+name+".db", 0600, nil)
	if err != nil {
		log.Fatalln(err)
	}
	return db
}

func InitIndex(tx *bolt.Tx, name string, offset uint64) error {
	key := []byte(name)
	if b, err := tx.CreateBucketIfNotExists([]byte("index")); err != nil {
		return err
	} else {
		s := string(b.Get(key))
		if len(s) == 0 {
			return b.Put(key, []byte(strconv.FormatUint(offset, 10)))
		}
	}
	return nil
}

func PutBucketBytes(tx *bolt.Tx, bucketName string, id string, value []byte) error {
	return tx.Bucket([]byte(bucketName)).Put([]byte(id), value)
}

func DelBucketBytes(tx *bolt.Tx, bucketName string, id string) error {
	return tx.Bucket([]byte(bucketName)).Delete([]byte(id))
}

func GetTxJson(tx *bolt.Tx, bucketName, key string, val interface{}) error {
	return json.Unmarshal(tx.Bucket([]byte(bucketName)).Get([]byte(key)), val)
}

func PutTxJson(tx *bolt.Tx, bucketName, key string, val interface{}) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return tx.Bucket([]byte(bucketName)).Put([]byte(key), b)
}

// increments index for the specified bucket using the given R/W transaction.
func GetNextIndex(tx *bolt.Tx, bucket string) (string, error) {
	var id uint64
	key := []byte(bucket)

	b := tx.Bucket([]byte("index"))
	s := string(b.Get(key))

	id, _ = strconv.ParseUint(s, 10, 64)
	b.Put(key, []byte(strconv.FormatUint(id+1, 10)))

	return strconv.FormatUint(id, 10), nil
}
