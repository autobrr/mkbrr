//go:build 386 || arm || mips || mipsle

package torrent

import (
	"sync/atomic"
)

type atomicCounter struct {
	count uint32
}

func (c *atomicCounter) Add(val uint64) {
	atomic.AddUint32(&c.count, uint32(val))
}

func (c *atomicCounter) Load() uint64 {
	return uint64(atomic.LoadUint32(&c.count))
}
