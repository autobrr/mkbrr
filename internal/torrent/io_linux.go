//go:build linux
// +build linux

package torrent

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioReader is an interface for reading files
type ioReader interface {
	Read(offset int64, buf []byte) (int, error)
	Close() error
}

// ioUringReader implements ioReader using io_uring when available
type ioUringReader struct {
	fd     int
	path   string
	closed bool
	ring   *ioURing
}

// ioURing represents an io_uring instance
type ioURing struct {
	fd         int
	sqRing     []byte
	cqRing     []byte
	sqEntries  []unix.IoUringSQE
	cqEntries  []unix.IoUringCQE
	sqHead     *uint32
	sqTail     *uint32
	sqMask     *uint32
	sqArray    *uint32
	cqHead     *uint32
	cqTail     *uint32
	cqMask     *uint32
	ringSize   uint32
	ringMask   uint32
	initialized bool
	mu         sync.Mutex
}

// Global io_uring instance for reuse
var (
	globalRing     *ioURing
	globalRingOnce sync.Once
)

// getGlobalRing returns a shared io_uring instance
func getGlobalRing() (*ioURing, error) {
	var initErr error
	globalRingOnce.Do(func() {
		globalRing = &ioURing{}
		initErr = globalRing.init(128) // 128 entries is a good default
	})
	return globalRing, initErr
}

// init initializes an io_uring instance
func (r *ioURing) init(entries uint32) error {
	// Set up io_uring
	params := unix.IoUringParams{}
	fd, err := unix.IoUringSetup(entries, &params)
	if err != nil {
		return fmt.Errorf("io_uring_setup failed: %w", err)
	}
	r.fd = fd
	r.ringSize = entries

	// Map submission queue
	sqSize := params.SqOff.Array + params.SqEntries*uint32(unsafe.Sizeof(uint32(0)))
	sqRing, err := unix.Mmap(fd, unix.IORING_OFF_SQ_RING, int(sqSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(fd)
		return fmt.Errorf("mmap sq_ring failed: %w", err)
	}
	r.sqRing = sqRing

	// Map completion queue
	cqSize := params.CqOff.Cqes + params.CqEntries*uint32(unsafe.Sizeof(unix.IoUringCQE{}))
	cqRing, err := unix.Mmap(fd, unix.IORING_OFF_CQ_RING, int(cqSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqRing)
		unix.Close(fd)
		return fmt.Errorf("mmap cq_ring failed: %w", err)
	}
	r.cqRing = cqRing

	// Map SQEs
	sqesSize := params.SqEntries * uint32(unsafe.Sizeof(unix.IoUringSQE{}))
	sqEntries, err := unix.Mmap(fd, unix.IORING_OFF_SQES, int(sqesSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(cqRing)
		unix.Munmap(sqRing)
		unix.Close(fd)
		return fmt.Errorf("mmap sqes failed: %w", err)
	}

	// Set up pointers
	r.sqHead = (*uint32)(unsafe.Pointer(&sqRing[params.SqOff.Head]))
	r.sqTail = (*uint32)(unsafe.Pointer(&sqRing[params.SqOff.Tail]))
	r.sqMask = (*uint32)(unsafe.Pointer(&sqRing[params.SqOff.Mask]))
	r.sqArray = (*uint32)(unsafe.Pointer(&sqRing[params.SqOff.Array]))
	r.cqHead = (*uint32)(unsafe.Pointer(&cqRing[params.CqOff.Head]))
	r.cqTail = (*uint32)(unsafe.Pointer(&cqRing[params.CqOff.Tail]))
	r.cqMask = (*uint32)(unsafe.Pointer(&cqRing[params.CqOff.Mask]))

	// Set up SQEs and CQEs
	r.sqEntries = make([]unix.IoUringSQE, params.SqEntries)
	for i := uint32(0); i < params.SqEntries; i++ {
		offset := i * uint32(unsafe.Sizeof(unix.IoUringSQE{}))
		r.sqEntries[i] = *(*unix.IoUringSQE)(unsafe.Pointer(&sqEntries[offset]))
	}

	r.cqEntries = make([]unix.IoUringCQE, params.CqEntries)
	for i := uint32(0); i < params.CqEntries; i++ {
		offset := i * uint32(unsafe.Sizeof(unix.IoUringCQE{}))
		r.cqEntries[i] = *(*unix.IoUringCQE)(unsafe.Pointer(&cqRing[params.CqOff.Cqes+offset]))
	}

	r.ringMask = *r.sqMask
	r.initialized = true
	return nil
}

// prepareRead prepares a read operation
func (r *ioURing) prepareRead(fd int, buf []byte, offset int64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return 0, fmt.Errorf("io_uring not initialized")
	}

	// Get next sqe
	head := *r.sqHead
	next := *r.sqTail
	if (next - head) == r.ringSize {
		return 0, fmt.Errorf("submission queue full")
	}

	index := next & r.ringMask
	sqe := &r.sqEntries[index]

	// Prepare read operation
	sqe.Opcode = unix.IORING_OP_READ
	sqe.Fd = int32(fd)
	sqe.Addr = (uint64)(uintptr(unsafe.Pointer(&buf[0])))
	sqe.Len = uint32(len(buf))
	sqe.OffsetLow = uint32(offset)
	sqe.OffsetHigh = uint32(offset >> 32)
	sqe.Flags = 0
	sqe.UserData = uint64(index) // Use index as user data for tracking

	// Update tail
	atomic.StoreUint32(r.sqTail, next+1)

	return uint64(index), nil
}

// submit submits the prepared operations
func (r *ioURing) submit() (int, error) {
	return unix.IoUringEnter(r.fd, 1, 1, unix.IORING_ENTER_GETEVENTS)
}

// waitCompletion waits for a completion event
func (r *ioURing) waitCompletion(userData uint64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		head := atomic.LoadUint32(r.cqHead)
		tail := atomic.LoadUint32(r.cqTail)

		if head == tail {
			// No completions available, wait
			_, err := unix.IoUringEnter(r.fd, 0, 1, unix.IORING_ENTER_GETEVENTS)
			if err != nil {
				return 0, err
			}
			continue
		}

		// Process completions
		for ; head != tail; head++ {
			index := head & *r.cqMask
			cqe := &r.cqEntries[index]

			if cqe.UserData == userData {
				// Found our completion
				if cqe.Res < 0 {
					err := syscall.Errno(-cqe.Res)
					atomic.StoreUint32(r.cqHead, head+1)
					return 0, err
				}

				atomic.StoreUint32(r.cqHead, head+1)
				return int(cqe.Res), nil
			}
		}

		// Update head
		atomic.StoreUint32(r.cqHead, head)
	}
}

// close closes the io_uring instance
func (r *ioURing) close() error {
	if !r.initialized {
		return nil
	}

	// Unmap memory
	if r.sqRing != nil {
		unix.Munmap(r.sqRing)
	}
	if r.cqRing != nil {
		unix.Munmap(r.cqRing)
	}

	// Close fd
	if r.fd != 0 {
		unix.Close(r.fd)
	}

	r.initialized = false
	return nil
}

// NewIOReader creates an appropriate IO reader based on system capabilities
func NewIOReader(path string) (ioReader, error) {
	// Try to use io_uring first
	if isIOUringSupported() {
		ring, err := getGlobalRing()
		if err == nil {
			fd, err := unix.Open(path, unix.O_RDONLY, 0)
			if err != nil {
				// Fall back to standard file IO
				f, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				return &stdReader{file: f}, nil
			}
			
			return &ioUringReader{
				fd:   fd,
				path: path,
				ring: ring,
			}, nil
		}
	}
	
	// Fall back to standard file IO
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &stdReader{file: f}, nil
}

// isIOUringSupported checks if io_uring is supported on this system
func isIOUringSupported() bool {
	// Try to create a minimal io_uring instance
	params := unix.IoUringParams{}
	fd, err := unix.IoUringSetup(4, &params)
	if err != nil {
		return false
	}
	unix.Close(fd)
	return true
}

// Read implements the ioReader interface using io_uring
func (r *ioUringReader) Read(offset int64, buf []byte) (int, error) {
	if r.closed {
		return 0, fmt.Errorf("file already closed")
	}

	// Prepare read operation
	userData, err := r.ring.prepareRead(r.fd, buf, offset)
	if err != nil {
		// Fall back to pread if io_uring preparation fails
		return unix.Pread(r.fd, buf, offset)
	}

	// Submit operation
	_, err = r.ring.submit()
	if err != nil {
		// Fall back to pread if submission fails
		return unix.Pread(r.fd, buf, offset)
	}

	// Wait for completion
	return r.ring.waitCompletion(userData)
}

// Close closes the file descriptor
func (r *ioUringReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return unix.Close(r.fd)
}

// stdReader implements ioReader using standard os.File
type stdReader struct {
	file *os.File
}

// Read implements the ioReader interface using standard file IO
func (r *stdReader) Read(offset int64, buf []byte) (int, error) {
	// Seek to the offset
	_, err := r.file.Seek(offset, 0)
	if err != nil {
		return 0, err
	}
	// Read the data
	return r.file.Read(buf)
}

// Close closes the file
func (r *stdReader) Close() error {
	return r.file.Close()
} 