package ringbuffer

import (
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

	m         *sync.RWMutex
	readWake  *sync.Cond
	writeWake *sync.Cond
}

func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:       make([]byte, size),
		m:         &sync.RWMutex{},
		readWake:  sync.NewCond(&sync.Mutex{}),
		writeWake: sync.NewCond(&sync.Mutex{}),
	}
}

func (r *RingBuffer) Read(p []byte) (int, error) {
	if r.isClosedRead() {
		return 0, r.returnState()
	}

	w := 0
	r.readWake.L.Lock()
	defer r.readWake.L.Unlock()
	for w < len(p) {
		if r.isShutdown() {
			return w, io.ErrUnexpectedEOF
		} else if r.isEmpty() {
			r.readWake.Wait()
			continue
		}

		r.m.Lock()
		if n := r.read(p[w:]); n != 0 {
			r.writeWake.Signal()
			w += n
		}
		r.m.Unlock()
	}

	var err error = nil
	if w != len(p) {
		err = io.ErrUnexpectedEOF
	}

	return w, err
}

func (r *RingBuffer) Write(p []byte) (int, error) {
	if r.isClosed() {
		return 0, r.returnState()
	}

	w := 0
	r.writeWake.L.Lock()
	defer r.writeWake.L.Unlock()
	for w < len(p) {
		if r.isShutdown() {
			return w, io.ErrUnexpectedEOF
		} else if r.isFull() {
			r.writeWake.Wait()
			continue
		}

		r.m.Lock()
		if n := r.write(p[w:]); n != 0 {
			r.readWake.Signal()
			w += n
		}
		r.m.Unlock()

	}

	var err error = nil
	if w != len(p) {
		err = io.ErrShortWrite
	}
	return w, err
}

func (r *RingBuffer) CloseWriter() {
	r.CloseWithError(io.EOF)
}

func (r *RingBuffer) CloseWithError(err error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.err = err
}

func (r *RingBuffer) Reset() {
	r.m.Lock()
	defer r.m.Unlock()
	r.shutdown = true
	r.readWake.Broadcast()
	r.writeWake.Broadcast()
	r.readWake.L.Lock()
	r.writeWake.L.Lock()
	defer r.readWake.L.Unlock()
	defer r.writeWake.L.Unlock()

	r.err = nil
	r.start = 0
	r.end = 0
	r.full = false
	r.shutdown = false
}

func (r *RingBuffer) Bytes() []byte {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.bytes()
}

func (r *RingBuffer) isEmpty() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.isempty()
}

func (r *RingBuffer) isFull() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.isfull()
}

func (r *RingBuffer) isClosedRead() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.isclosed() && r.isempty()
}

func (r *RingBuffer) isClosed() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.isclosed()
}

func (r *RingBuffer) isShutdown() bool {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.shutdown
}

func (r *RingBuffer) returnState() error {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.err
}

func (r *RingBuffer) bytes() []byte {
	if r.isempty() || r.start <= r.end {
		//fmt.Printf("seq: first %q %d second %q %d final %d\n", r.buf[r.start:], r.start, r.buf[:r.end], r.end, len(r.buf))
		return r.buf[r.start:r.end]
	}

	v := make([]byte, 0, len(r.buf)-(r.start-r.end))
	v = append(v, r.buf[r.start:]...)
	v = append(v, r.buf[:r.end]...)
	//fmt.Printf("bytes: first %q %d second %q %d final %q %d\n", r.buf[r.start:], r.start, r.buf[:r.end], r.end, v, len(r.buf))
	return v
}

func (r *RingBuffer) read(p []byte) int {
	w := 0
	//fmt.Printf("Read: %q | Buf: %q | start %d | end %d\n", p, r.bytes(), r.start, r.end)
	if r.start < r.end {
		end := min(r.end-r.start, len(p))
		w = copy(p, r.buf[r.start:r.start+end])
		r.start += end
	} else {
		end := min(len(r.buf)-r.start, len(p))
		w = copy(p, r.buf[r.start:r.start+end])
		r.start = (r.start + end) % len(r.buf)
	}

	r.full = r.full && w == 0
	//fmt.Printf("Read Done: %q | Buf: %q | start %d | end %d | full %v\n", p, r.bytes(), r.start, r.end, r.full)
	return w
}

func (r *RingBuffer) write(p []byte) int {
	w := 0
	//fmt.Printf("Write: %q | Buf: %q | start %d | end %d\n", p, r.bytes(), r.start, r.end)
	if r.start <= r.end {
		end := min(len(r.buf)-r.end, len(p))
		w = copy(r.buf[r.end:r.end+end], p)
		r.end = (r.end + end) % len(r.buf)
	} else {
		// Write to the start of the buffer
		end := min(r.start-r.end, len(p))
		w = copy(r.buf[r.end:r.end+end], p)
		r.end += end
	}

	r.full = w != 0 && r.end == r.start
	r.bytes()
	//fmt.Printf("Write Done: %q | Buf: %q | start %d | end %d | full %v\n", p, r.bytes(), r.start, r.end, r.full)
	return w
}

func (r *RingBuffer) isclosed() bool {
	return r.err != nil
}

func (r *RingBuffer) isempty() bool {
	return !r.isfull() && r.start == r.end
}

func (r *RingBuffer) isfull() bool {
	return r.full
}
