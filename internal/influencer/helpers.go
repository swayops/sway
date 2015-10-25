package influencer

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"
	"unsafe"
)

func pseudoUUID() string {
	now := time.Now().UnixNano()
	randPart := make([]byte, 7)
	if _, err := rand.Read(randPart); err != nil {
		copy(randPart, (*(*[8]byte)(unsafe.Pointer(&now)))[:7])
	}
	return strconv.FormatInt(now, 10)[1:] + hex.EncodeToString(randPart)
}
