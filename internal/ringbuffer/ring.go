package ringbuffer

import (
	"fmt"
	"io"
	"sync"
)

type RingBuffer struct {
	start int
	end   int
	buf   []byte

	full     bool
	shutdown bool

	err error

	m         *sync.Mutex
	readWake  *sync.Cond
	writeWake *sync.Cond
}

func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:       make([]byte, size),
		m:         &sync.Mutex{},
		readWake:  sync.NewCond(&sync.Mutex{}),
		writeWake: sync.NewCond(&sync.Mutex{}),
	}
}

func (r *RingBuffer) Read(p []byte) (int, error) {
	if r.err != nil && r.isEmpty() {
		return 0, r.err
	}

	r.readWake.L.Lock()
	for r.isEmpty() {
		r.readWake.Wait()
		if r.shutdown {
			r.readWake.L.Unlock()
			return 0, io.ErrUnexpectedEOF
		}
	}
	defer r.readWake.L.Unlock()

	r.m.Lock()
	n := r.read(p)
	r.m.Unlock()
	r.writeWake.Signal()
	return n, nil
}

func (r *RingBuffer) Write(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	r.writeWake.L.Lock()
	for r.isFull() {
		r.writeWake.Wait()
		if r.shutdown {
			r.readWake.L.Unlock()
			return 0, io.ErrUnexpectedEOF
		}
	}
	defer r.writeWake.L.Unlock()

	r.m.Lock()
	n := r.write(p)
	r.m.Unlock()
	r.readWake.Signal()
	return n, nil
}

func (r *RingBuffer) CloseWriter() {
	r.CloseWithError(io.EOF)
}

func (r *RingBuffer) CloseWithError(err error) {
	r.m.Lock()
	r.err = err
	r.m.Unlock()
}

func (r *RingBuffer) Reset() {
	r.m.Lock()
	r.shutdown = true
	r.readWake.Broadcast()
	r.writeWake.Broadcast()
	r.readWake.L.Lock()
	r.writeWake.L.Lock()

	r.err = nil
	r.start = 0
	r.end = 0
	r.full = false
	r.shutdown = false

	r.readWake.L.Unlock()
	r.writeWake.L.Unlock()
	r.m.Unlock()
	fmt.Printf("RESET DONE!\n")
}

func (r *RingBuffer) read(p []byte) int {
	w := 0
	for w < len(p) && !r.isEmpty() {
		if r.start < r.end {
			end := min(r.end-r.start, len(p)-w)
			w += copy(p[w:], r.buf[r.start:r.start+end])
			r.start += end
		} else {
			end := min(len(r.buf)-r.start, len(p)-w)
			w += copy(p[w:], r.buf[r.start:r.start+end])
			r.start = (r.start + end) % len(r.buf)
		}
	}
	r.full = false
	return w
}

func (r *RingBuffer) write(p []byte) int {
	w := 0
	for w < len(p) && !r.isFull() {
		if r.end >= r.start {
			end := min(len(r.buf)-r.end, len(p)-w)
			w += copy(r.buf[r.end:r.end+end], p[w:])
			r.end = (r.end + end) % len(r.buf)
		} else {
			// Write to the start of the buffer
			end := min(r.start-r.end, len(p)-w)
			w += copy(r.buf[r.end:r.end+end], p[w:])
			r.end += end
		}
	}

	if w > 0 {
		r.full = r.end == r.start
	}
	return w
}

func (r *RingBuffer) bytes() []byte {
	r.m.Lock()
	defer r.m.Unlock()

	if r.isEmpty() {
		return nil
	}

	if r.start < r.end {
		return r.buf[r.start:r.end]
	}

	v := make([]byte, len(r.buf)-(r.start-r.end))
	copy(v, r.buf[r.start:])
	copy(v[len(r.buf)-r.start:], r.buf[:r.end])
	return v
}

func (r *RingBuffer) isEmpty() bool {
	return !r.isFull() && r.start == r.end
}

func (r *RingBuffer) isFull() bool {
	return r.full
}
