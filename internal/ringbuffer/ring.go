package ringbuffer

import (
	"io"
	"sync"
)

type RingBuffer struct {
	err error

	m         *sync.RWMutex
	readWake  *sync.Cond
	writeWake *sync.Cond
	buf       []byte

	start int
	end   int

	full     bool
	shutdown bool
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
		} else if r.isClosedRead() {
			break
		} else if r.isEmpty() {
			if w != 0 {
				break
			}
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

	return w, nil
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
		} else if r.isClosed() {
			break
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

func (r *RingBuffer) ReadFrom(rio io.Reader) (int64, error) {
	if r.isClosed() {
		return 0, r.returnState()
	}

	var w int64
	r.writeWake.L.Lock()
	defer r.writeWake.L.Unlock()
	for {
		if r.isShutdown() {
			return w, io.ErrUnexpectedEOF
		} else if r.isClosed() {
			break
		} else if r.isFull() {
			r.writeWake.Wait()
			continue
		}

		r.m.Lock()
		nn, e := r.copyfrom(rio)
		r.m.Unlock()

		if nn > 0 {
			w += int64(nn)
			r.readWake.Signal()
		}

		if e != nil {
			if e == io.EOF {
				break
			}
			return w, e
		}
	}

	//fmt.Printf("ReadFrom Done: W %d | Buf: %q | start %d | end %d | full %v\n", w, r.bytes(), r.start, r.end, r.full)
	return w, nil
}

func (r *RingBuffer) WriteTo(wio io.Writer) (int64, error) {
	if r.isClosedRead() {
		return 0, r.returnState()
	}

	var w int64
	r.readWake.L.Lock()
	defer r.readWake.L.Unlock()
	for {
		if r.isShutdown() {
			return w, io.ErrUnexpectedEOF
		} else if r.isClosedRead() {
			break
		} else if r.isEmpty() {
			r.readWake.Wait()
			continue
		}

		r.m.Lock()
		nn, e := r.copyto(wio)
		r.m.Unlock()

		if nn > 0 {
			w += int64(nn)
			r.writeWake.Signal()
		} else if nn == 0 && e == nil {
			break
		}

		if e != nil {
			if e == io.EOF {
				break
			}
			return w, e
		}
	}

	return w, nil
}

func (r *RingBuffer) Len() int {
	r.m.RLock()
	defer r.m.RUnlock()

	if r.start <= r.end {
		return r.end - r.start
	}

	return len(r.buf) - r.start + r.end
}

func (r *RingBuffer) Size() int {
	return len(r.buf)
}

func (r *RingBuffer) CloseWriter() {
	r.CloseWithError(io.EOF)
}

func (r *RingBuffer) CloseWithError(err error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.err = err
	r.readWake.Signal()
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
	r.resetposition()
	r.shutdown = false
}

func (r *RingBuffer) Bytes() []byte {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.bytes()
}

func (r *RingBuffer) ConsumeBytes() []byte {
	r.m.Lock()
	defer r.m.Unlock()
	defer r.resetposition()

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

func processRing[T any](p []T, src []T, start, end int) int {
	if start < end {
		return copy(p, src[start:end])
	}

	n := copy(p, src[start:])
	if n < len(p) {
		n += copy(p[n:], src[:end])
	}
	return n
}

func (r *RingBuffer) read(p []byte) int {
	w := processRing(p, r.buf, r.start, r.end)
	r.recalculateRead(w)
	return w
}

func (r *RingBuffer) write(p []byte) int {
	w := processRing(r.buf[r.end:], p, 0, len(p))
	r.recalculateWrite(w)
	return w
}

func processIO(buf []byte, start, end int, handler func([]byte) (int, error)) (int, error) {
	if start < end {
		return handler(buf[start:end])
	}

	n, err := handler(buf[start:])
	if err != nil || n == len(buf)-start {
		return n, err
	}

	m, err := handler(buf[:end])
	return n + m, err
}

func (r *RingBuffer) copyfrom(rio io.Reader) (int, error) {
	w, err := processIO(r.buf, r.end, r.start, rio.Read)
	r.recalculateWrite(w)
	return w, err
}

func (r *RingBuffer) copyto(wio io.Writer) (int, error) {
	w, err := processIO(r.buf, r.start, r.end, wio.Write)
	r.recalculateRead(w)
	return w, err
}

func (r *RingBuffer) recalculateRead(w int) {
	r.start = (r.start + w) % len(r.buf)
	r.full = r.full && w == 0
	if r.isempty() {
		r.resetposition()
	}
}

func (r *RingBuffer) recalculateWrite(w int) {
	r.end = (r.end + w) % len(r.buf)
	r.full = w != 0 && r.end == r.start
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

func (r *RingBuffer) resetposition() {
	r.start = 0
	r.end = 0
	r.full = false
}
