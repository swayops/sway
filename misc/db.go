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
	b, err := tx.CreateBucketIfNotExists([]byte("index"))
	if err != nil {
		return err
	}
	if key := []byte(name); len(b.Get(key)) == 0 {
		return b.Put(key, big.NewInt(int64(offset)).Bytes())
	}
	return nil
}

func GetBucket(tx *bolt.Tx, bucketName string) *bolt.Bucket {
	return tx.Bucket([]byte(bucketName))
}

func PutBucketBytes(tx *bolt.Tx, bucketName string, id string, value []byte) error {
	return GetBucket(tx, bucketName).Put([]byte(id), value)
}

func DelBucketBytes(tx *bolt.Tx, bucketName string, id string) error {
	return GetBucket(tx, bucketName).Delete([]byte(id))
}

func GetTxJson(tx *bolt.Tx, bucketName, key string, val interface{}) error {
	return json.Unmarshal(GetBucket(tx, bucketName).Get([]byte(key)), val)
}

func PutTxJson(tx *bolt.Tx, bucketName, key string, val interface{}) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return GetBucket(tx, bucketName).Put([]byte(key), b)
}

var one = big.NewInt(1)

// increments index for the specified bucket using the given R/W transaction.
func GetNextIndex(tx *bolt.Tx, bucket string) (string, error) {
	key := []byte(bucket)
	// note that using SetBytes is pure bytes not  the string rep of the number.
	b := GetBucket(tx, "index")
	ov := b.Get(key)
	n := new(big.Int).SetBytes(ov)
	nv := new(big.Int).Add(n, one).Bytes()
	b.Put(key, nv)
	return n.String(), nil
}
