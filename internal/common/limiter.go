package common

import (
	"sync"
	"time"

	"github.com/swayops/sway/misc"
)

type LimitSet struct {
	m map[string][]int32
	l sync.RWMutex
}

func NewLimitSet() *LimitSet {
	c := &LimitSet{m: make(map[string][]int32)}
	return c
}

func (ls *LimitSet) Set(ip string) {
	ls.l.Lock()
	ls.m[ip] = append(ls.m[ip], int32(time.Now().Unix()))
	ls.l.Unlock()
}

func (ls *LimitSet) IsAllowed(ip string) (waitingPeriod string, ok bool) {
	ls.l.Lock()
	v, ok := ls.m[ip]
	ls.l.Unlock()

	if !ok || len(v) == 0 {
		// IP wasn't found!
		return "", true
	}

	var hour, day int32
	for _, ts := range v {
		if misc.WithinLast(ts, 1) {
			hour += 1
		}

		if misc.WithinLast(ts, 24) {
			day += 1
		}
	}

	if hour > 30 {
		return "1 hour", false
	}

	if day > 100 {
		return "24 hours", false
	}

	return "", true
}
