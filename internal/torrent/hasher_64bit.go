//go:build amd64 || arm64 || ppc64 || ppc64le || s390x || mips64 || mips64le

package torrent

import (
	"sync/atomic"
)

type atomicCounter struct {
	count uint64
}

func (c *atomicCounter) Add(val uint64) {
	atomic.AddUint64(&c.count, val)
}

func (c *atomicCounter) Load() uint64 {
	return atomic.LoadUint64(&c.count)
}
