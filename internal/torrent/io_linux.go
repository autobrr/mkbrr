//go:build linux
// +build linux

package torrent

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// ioReader is an interface for reading files
type ioReader interface {
	Read(offset int64, buf []byte) (int, error)
	Close() error
}

// io_uring constants - these are architecture-independent
const (
	// Operation codes
	IORING_OP_NOP          = 0
	IORING_OP_READV        = 1
	IORING_OP_WRITEV       = 2
	IORING_OP_FSYNC        = 3
	IORING_OP_READ_FIXED   = 4
	IORING_OP_WRITE_FIXED  = 5
	IORING_OP_POLL_ADD     = 6
	IORING_OP_POLL_REMOVE  = 7
	IORING_OP_SYNC_FILE_RANGE = 8
	IORING_OP_SENDMSG      = 9
	IORING_OP_RECVMSG      = 10
	IORING_OP_TIMEOUT      = 11
	IORING_OP_TIMEOUT_REMOVE = 12
	IORING_OP_ACCEPT       = 13
	IORING_OP_ASYNC_CANCEL = 14
	IORING_OP_LINK_TIMEOUT = 15
	IORING_OP_CONNECT      = 16
	IORING_OP_FALLOCATE    = 17
	IORING_OP_OPENAT       = 18
	IORING_OP_CLOSE        = 19
	IORING_OP_FILES_UPDATE = 20
	IORING_OP_STATX        = 21
	IORING_OP_READ         = 22
	IORING_OP_WRITE        = 23
	IORING_OP_FADVISE      = 24
	IORING_OP_MADVISE      = 25
	IORING_OP_SEND         = 26
	IORING_OP_RECV         = 27
	IORING_OP_OPENAT2      = 28
	IORING_OP_EPOLL_CTL    = 29
	IORING_OP_SPLICE       = 30
	IORING_OP_PROVIDE_BUFFERS = 31
	IORING_OP_REMOVE_BUFFERS  = 32
	IORING_OP_TEE             = 33
	IORING_OP_SHUTDOWN        = 34
	IORING_OP_RENAMEAT        = 35
	IORING_OP_UNLINKAT        = 36

	// Flags
	IORING_ENTER_GETEVENTS = 1
	IORING_ENTER_SQ_WAKEUP = 2
	
	// Offsets
	IORING_OFF_SQ_RING  = 0
	IORING_OFF_CQ_RING  = 0x8000000
	IORING_OFF_SQES     = 0x10000000
	
	// Timeout constants
	ioUringOpTimeout = 100 * time.Millisecond
)

// io_uring setup syscall numbers for different architectures
var ioUringSetupSyscallNum = map[string]uintptr{
	"386":         425,
	"amd64":       425,
	"arm":         425,
	"arm64":       425,
	"ppc64":       425,
	"ppc64le":     425,
	"mips":        425,
	"mipsle":      425,
	"mips64":      425,
	"mips64le":    425,
	"s390x":       425,
	"riscv64":     425,
}

// io_uring enter syscall numbers for different architectures
var ioUringEnterSyscallNum = map[string]uintptr{
	"386":         426,
	"amd64":       426,
	"arm":         426,
	"arm64":       426,
	"ppc64":       426,
	"ppc64le":     426,
	"mips":        426,
	"mipsle":      426,
	"mips64":      426,
	"mips64le":    426,
	"s390x":       426,
	"riscv64":     426,
}

// io_uring structures
type ioUringParams struct {
	sqEntries    uint32
	cqEntries    uint32
	flags        uint32
	sqThreadCpu  uint32
	sqThreadIdle uint32
	features     uint32
	wqFd         uint32
	resv         [3]uint32
	sqOff        ioUringSQRingOffsets
	cqOff        ioUringCQRingOffsets
}

type ioUringSQRingOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	flags       uint32
	dropped     uint32
	array       uint32
	resv1       uint32
	resv2       uint64
}

type ioUringCQRingOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	overflow    uint32
	cqes        uint32
	flags       uint32
	resv1       uint32
	resv2       uint64
}

type ioUringSQE struct {
	opcode      uint8
	flags       uint8
	ioprio      uint16
	fd          int32
	off         uint64
	addr        uint64
	len         uint32
	opcodeFlags uint32
	userData    uint64
	_pad        [3]uint64
}

type ioUringCQE struct {
	userData uint64
	res      int32
	flags    uint32
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
	fd          int
	params      ioUringParams
	sqRing      []byte
	cqRing      []byte
	sqes        []byte
	sqHead      *uint32
	sqTail      *uint32
	sqMask      *uint32
	sqArray     []uint32
	cqHead      *uint32
	cqTail      *uint32
	cqMask      *uint32
	entries     uint32
	initialized bool
	mu          sync.Mutex
}

// Global io_uring instance for reuse
var (
	globalRing     *ioURing
	globalRingOnce sync.Once
	ringSupported  bool
	disableIoUring atomic.Bool // Track if io_uring should be disabled due to errors
)

func init() {
	// Start with io_uring disabled until we confirm it works
	disableIoUring.Store(true)
}

// io_uring_setup syscall using architecture-specific syscall number
func ioUringSetup(entries uint32, params *ioUringParams) (int, error) {
	arch := runtime.GOARCH
	syscallNum, ok := ioUringSetupSyscallNum[arch]
	if !ok {
		return 0, syscall.ENOSYS
	}
	
	r1, _, errno := syscall.Syscall(syscallNum, uintptr(entries), uintptr(unsafe.Pointer(params)), 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// io_uring_enter syscall using architecture-specific syscall number
func ioUringEnter(fd int, toSubmit uint32, minComplete uint32, flags uint32, sigset unsafe.Pointer, size uint32) (int, error) {
	arch := runtime.GOARCH
	syscallNum, ok := ioUringEnterSyscallNum[arch]
	if !ok {
		return 0, syscall.ENOSYS
	}
	
	r1, _, errno := syscall.Syscall6(syscallNum, uintptr(fd), uintptr(toSubmit), uintptr(minComplete), 
	                                uintptr(flags), uintptr(sigset), uintptr(size))
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// checkIoURingSupport tests if io_uring is properly supported
func checkIoURingSupport() bool {
	// Try to create a minimal io_uring instance
	params := ioUringParams{}
	fd, err := ioUringSetup(4, &params)
	if err != nil {
		return false
	}
	// Clean up
	unix.Close(fd)
	return true
}

// getGlobalRing returns a shared io_uring instance
func getGlobalRing() (*ioURing, error) {
	globalRingOnce.Do(func() {
		// First check if io_uring is supported at all
		if !checkIoURingSupport() {
			ringSupported = false
			return
		}
		
		ring := &ioURing{}
		err := ring.init(128) // 128 entries is a good default
		if err == nil {
			globalRing = ring
			ringSupported = true
			// Only enable io_uring if initialization succeeded
			disableIoUring.Store(false)
		} else {
			ringSupported = false
		}
	})
	
	if !ringSupported {
		return nil, syscall.ENOSYS
	}
	return globalRing, nil
}

// init initializes an io_uring instance
func (r *ioURing) init(entries uint32) error {
	// Set up io_uring
	params := ioUringParams{}
	fd, err := ioUringSetup(entries, &params)
	if err != nil {
		return err
	}
	r.fd = fd
	r.params = params
	r.entries = entries

	// Map submission queue
	sqSize := params.sqOff.array + params.sqEntries*4
	sqRing, err := unix.Mmap(fd, IORING_OFF_SQ_RING, int(sqSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(fd)
		return err
	}
	r.sqRing = sqRing

	// Map completion queue
	cqSize := params.cqOff.cqes + params.cqEntries*16
	cqRing, err := unix.Mmap(fd, IORING_OFF_CQ_RING, int(cqSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqRing)
		unix.Close(fd)
		return err
	}
	r.cqRing = cqRing

	// Map SQEs
	sqesSize := params.sqEntries * 64
	sqes, err := unix.Mmap(fd, IORING_OFF_SQES, int(sqesSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(cqRing)
		unix.Munmap(sqRing)
		unix.Close(fd)
		return err
	}
	r.sqes = sqes

	// Set up pointers
	r.sqHead = (*uint32)(unsafe.Pointer(&sqRing[params.sqOff.head]))
	r.sqTail = (*uint32)(unsafe.Pointer(&sqRing[params.sqOff.tail]))
	r.sqMask = (*uint32)(unsafe.Pointer(&sqRing[params.sqOff.ringMask]))
	
	// Set up the array properly - this is the robust replacement for the previous unsafe array
	arrayPtr := (*uint32)(unsafe.Pointer(&sqRing[params.sqOff.array]))
	r.sqArray = make([]uint32, params.sqEntries)
	for i := uint32(0); i < params.sqEntries; i++ {
		r.sqArray[i] = *(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(arrayPtr)) + uintptr(i)*unsafe.Sizeof(uint32(0))))
	}
	
	r.cqHead = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.head]))
	r.cqTail = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.tail]))
	r.cqMask = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.ringMask]))
	
	r.initialized = true
	return nil
}

// prepareRead prepares a read operation
func (r *ioURing) prepareRead(fd int, buf []byte, offset int64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return 0, syscall.EINVAL
	}

	// Get next sqe
	head := *r.sqHead
	next := *r.sqTail
	if (next - head) == r.params.sqEntries {
		return 0, syscall.EBUSY
	}

	index := next & *r.sqMask
	if index >= r.params.sqEntries {
		return 0, syscall.EINVAL
	}
	
	sqeOffset := index * 64
	
	// Prepare read operation
	sqe := (*ioUringSQE)(unsafe.Pointer(&r.sqes[sqeOffset]))
	sqe.opcode = IORING_OP_READ
	sqe.flags = 0
	sqe.ioprio = 0
	sqe.fd = int32(fd)
	sqe.off = uint64(offset)
	sqe.addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
	sqe.opcodeFlags = 0
	
	// Use a unique userData to identify this operation
	// Combine index with a counter to ensure uniqueness
	userData := uint64(time.Now().UnixNano()) | (uint64(index) << 56)
	sqe.userData = userData

	// Update array properly using our properly initialized array
	sqArrayPtr := (*uint32)(unsafe.Pointer(&r.sqRing[r.params.sqOff.array]))
	if sqArrayPtr != nil {
		*(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(sqArrayPtr)) + uintptr(index)*4)) = index
	}
	
	// Update tail with memory barrier to ensure visibility
	atomic.StoreUint32(r.sqTail, next+1)

	return userData, nil
}

// submit submits the prepared operations with timeout
func (r *ioURing) submit() (int, error) {
	// Add a timeout to prevent hangs
	submitChan := make(chan struct {
		n   int
		err error
	}, 1)
	
	go func() {
		n, err := ioUringEnter(r.fd, 1, 1, IORING_ENTER_GETEVENTS, nil, 0)
		submitChan <- struct {
			n   int
			err error
		}{n, err}
	}()
	
	select {
	case res := <-submitChan:
		return res.n, res.err
	case <-time.After(ioUringOpTimeout):
		return 0, syscall.ETIMEDOUT
	}
}

// waitCompletion waits for a completion event with timeout
func (r *ioURing) waitCompletion(userData uint64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	resultChan := make(chan struct {
		n   int
		err error
	}, 1)
	
	go func() {
		deadline := time.Now().Add(ioUringOpTimeout)
		
		for time.Now().Before(deadline) {
			head := atomic.LoadUint32(r.cqHead)
			tail := atomic.LoadUint32(r.cqTail)

			if head == tail {
				// No completions available, wait a bit
				_, err := ioUringEnter(r.fd, 0, 1, IORING_ENTER_GETEVENTS, nil, 0)
				if err != nil {
					resultChan <- struct {
						n   int
						err error
					}{0, err}
					return
				}
				time.Sleep(time.Millisecond)
				continue
			}

			// Process completions
			for ; head != tail; head++ {
				index := head & *r.cqMask
				if index >= r.params.cqEntries {
					resultChan <- struct {
						n   int
						err error
					}{0, syscall.EINVAL}
					return
				}
				
				cqeOffset := r.params.cqOff.cqes + index*16
				cqe := (*ioUringCQE)(unsafe.Pointer(&r.cqRing[cqeOffset]))

				if cqe.userData == userData {
					// Found our completion
					if cqe.res < 0 {
						err := syscall.Errno(-cqe.res)
						atomic.StoreUint32(r.cqHead, head+1)
						resultChan <- struct {
							n   int
							err error
						}{0, err}
						return
					}

					atomic.StoreUint32(r.cqHead, head+1)
					resultChan <- struct {
						n   int
						err error
					}{int(cqe.res), nil}
					return
				}
			}

			// Update head with memory barrier
			atomic.StoreUint32(r.cqHead, head)
		}
		
		// Timeout reached
		resultChan <- struct {
			n   int
			err error
		}{0, syscall.ETIMEDOUT}
	}()
	
	select {
	case res := <-resultChan:
		return res.n, res.err
	case <-time.After(ioUringOpTimeout * 2):
		return 0, syscall.ETIMEDOUT
	}
}

// close closes the io_uring instance
func (r *ioURing) close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if !r.initialized {
		return nil
	}
	
	r.initialized = false
	
	if r.sqRing != nil {
		unix.Munmap(r.sqRing)
		r.sqRing = nil
	}
	if r.cqRing != nil {
		unix.Munmap(r.cqRing)
		r.cqRing = nil
	}
	if r.sqes != nil {
		unix.Munmap(r.sqes)
		r.sqes = nil
	}
	if r.fd != 0 {
		unix.Close(r.fd)
		r.fd = 0
	}
	return nil
}

// NewIOReader creates an appropriate IO reader based on system capabilities
func NewIOReader(path string) (ioReader, error) {
	// Skip io_uring if it's been disabled due to previous errors
	if !disableIoUring.Load() {
		ring, err := getGlobalRing()
		if err == nil {
			fd, err := unix.Open(path, unix.O_RDONLY, 0)
			if err == nil {
				return &ioUringReader{
					fd:   fd,
					path: path,
					ring: ring,
				}, nil
			}
		}
	}
	
	// Try direct I/O next
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECT, 0)
	if err == nil {
		return &directReader{file: f}, nil
	}
	
	// Fall back to standard file IO
	f, err = os.Open(path)
	if err != nil {
		return nil, err
	}
	return &stdReader{file: f}, nil
}

// Read implements the ioReader interface using io_uring
func (r *ioUringReader) Read(offset int64, buf []byte) (int, error) {
	if r.closed {
		return 0, syscall.EBADF
	}

	// First try with io_uring
	userData, err := r.ring.prepareRead(r.fd, buf, offset)
	if err != nil {
		// Disable io_uring for future operations
		disableIoUring.Store(true)
		// Fall back to pread
		return unix.Pread(r.fd, buf, offset)
	}

	// Submit operation
	_, err = r.ring.submit()
	if err != nil {
		// Disable io_uring for future operations
		disableIoUring.Store(true)
		// Fall back to pread
		return unix.Pread(r.fd, buf, offset)
	}

	// Wait for completion
	n, err := r.ring.waitCompletion(userData)
	if err != nil {
		// Disable io_uring for future operations
		disableIoUring.Store(true)
		// Fall back to pread
		return unix.Pread(r.fd, buf, offset)
	}

	return n, nil
}

// Close closes the file descriptor
func (r *ioUringReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return unix.Close(r.fd)
}

// directReader implements ioReader using direct I/O
type directReader struct {
	file *os.File
	mu   sync.Mutex
}

// Read implements the ioReader interface using direct I/O
func (r *directReader) Read(offset int64, buf []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Use pread for direct reading at offset
	return unix.Pread(int(r.file.Fd()), buf, offset)
}

// Close closes the file
func (r *directReader) Close() error {
	return r.file.Close()
}

// stdReader implements ioReader using standard os.File
type stdReader struct {
	file *os.File
	mu   sync.Mutex
}

// Read implements the ioReader interface using standard file IO
func (r *stdReader) Read(offset int64, buf []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
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