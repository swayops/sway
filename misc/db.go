package misc

import (
	"encoding/json"
	"log"
	"math/big"

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
			return b.Put(key, big.NewInt(int64(offset)).Bytes())
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

var one = big.NewInt(1)

// increments index for the specified bucket using the given R/W transaction.
func GetNextIndex(tx *bolt.Tx, bucket string) (string, error) {
	key := []byte(bucket)
	// note that using SetBytes is pure bytes not  the string rep of the number.
	b := tx.Bucket([]byte("index"))
	n := new(big.Int).SetBytes(b.Get(key))
	return n.String(), b.Put(key, n.Add(n, one).Bytes())
}
