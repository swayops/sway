package misc

import (
	"encoding/json"
	"log"
	"math/big"
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

var one = new(big.Int).SetUint64(1)

// increments index for the specified bucket using the given R/W transaction.
func GetNextIndex(tx *bolt.Tx, bucket string) (string, error) {
	key := []byte(bucket)
	// note that using SetBytes is pure bytes not  the string rep of the number.
	b := tx.Bucket([]byte("index"))

	n := new(big.Int).SetBytes(b.Get(key))
	b.Put(key, new(big.Int).Add(n, one).Bytes())
	return n.String(), nil
}
