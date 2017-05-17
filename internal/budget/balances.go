package budget

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/boltdb/bolt"
	"github.com/swayops/sway/config"
	"github.com/swayops/sway/misc"
)

var ErrBalance = errors.New("Not enough balance")

func IncrBalance(advid string, amount float64, tx *bolt.Tx, cfg *config.Config) error {
	b := tx.Bucket([]byte(cfg.Bucket.Balance)).Get([]byte(advid))

	balance := Float64frombytes(b) + amount
	if err := misc.PutBucketBytes(tx, cfg.Bucket.Balance, advid, Float64bytes(balance)); err != nil {
		return err
	}

	return nil
}

func DeductBalance(advid string, amount float64, tx *bolt.Tx, cfg *config.Config) error {
	b := tx.Bucket([]byte(cfg.Bucket.Balance)).Get([]byte(advid))

	balance := Float64frombytes(b) - amount
	if balance < 0 {
		return ErrBalance
	}

	if err := misc.PutBucketBytes(tx, cfg.Bucket.Balance, advid, Float64bytes(balance)); err != nil {
		return err
	}

	return nil
}

func GetBalance(advid string, tx *bolt.Tx, cfg *config.Config) float64 {
	v := tx.Bucket([]byte(cfg.Bucket.Balance)).Get([]byte(advid))
	return Float64frombytes(v)
}

func Float64frombytes(bytes []byte) float64 {
	if len(bytes) == 0 {
		return 0
	}

	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

func Float64bytes(float float64) []byte {
	bits := math.Float64bits(float)
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, bits)
	return bytes
}
