package common

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	UPDATE     = 30 * time.Minute
	FOUR_HOURS = 4 * 60 * 60 // 4 hrs in secs
)

type Set struct {
	m map[string]int32
	l sync.RWMutex
}

func NewSet() *Set {
	c := &Set{m: make(map[string]int32)}
	c.clean()
	return c
}

func (s *Set) Set(ip, ua string) {
	// Allowing empty IP and UA because if someone is coming in with
	// empty values that's shady as hell too.. lets cap them at 1 too!
	key := FromIPAndUserAgent(ip, ua)

	s.l.Lock()
	s.m[key] = int32(time.Now().Unix())
	s.l.Unlock()
}

func (s *Set) clean() {
	// Every 30 minutes clear out any values that are older than 4 hours
	ticker := time.NewTicker(UPDATE)
	go func() {
		for range ticker.C {
			now := int32(time.Now().Unix())
			s.l.Lock()
			for key, ts := range s.m {
				if now > ts+FOUR_HOURS {
					delete(s.m, key)
				}
			}
			s.l.Unlock()
		}
	}()
}

func (s *Set) Exists(ip, ua string) bool {
	key := FromIPAndUserAgent(ip, ua)
	s.l.Lock()
	_, ok := s.m[key]
	s.l.Unlock()

	return ok
}

func FromIPAndUserAgent(ip, ua string) string {
	if idx := strings.Index(ip, ":"); idx > 0 { // strip the port if it exists
		ip = ip[:idx]
	}
	h := sha1.New()
	io.WriteString(h, ip)
	io.WriteString(h, ua)
	return hex.EncodeToString(h.Sum(nil))
}
