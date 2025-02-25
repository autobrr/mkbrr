//go:build linux
// +build linux

package torrent

import (
	"os"
	"runtime"
	"sync"
	"syscall"
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
	fd         int
	params     ioUringParams
	sqRing     []byte
	cqRing     []byte
	sqes       []byte
	sqHead     *uint32
	sqTail     *uint32
	sqMask     *uint32
	sqArray    *uint32
	cqHead     *uint32
	cqTail     *uint32
	cqMask     *uint32
	mu         sync.Mutex
}

// Global io_uring instance for reuse
var (
	globalRing     *ioURing
	globalRingOnce sync.Once
	ringSupported  bool
	disableIoUring atomic.Bool // Track if io_uring should be disabled due to errors
)

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
func ioUringEnter(fd int, toSubmit uint32, minComplete uint32, flags uint32) (int, error) {
	arch := runtime.GOARCH
	syscallNum, ok := ioUringEnterSyscallNum[arch]
	if !ok {
		return 0, syscall.ENOSYS
	}
	
	r1, _, errno := syscall.Syscall6(syscallNum, uintptr(fd), uintptr(toSubmit), uintptr(minComplete), uintptr(flags), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// getGlobalRing returns a shared io_uring instance
func getGlobalRing() (*ioURing, error) {
	globalRingOnce.Do(func() {
		ring := &ioURing{}
		err := ring.init(128) // 128 entries is a good default
		if err == nil {
			globalRing = ring
			ringSupported = true
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
	r.sqArray = (*uint32)(unsafe.Pointer(&sqRing[params.sqOff.array]))
	r.cqHead = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.head]))
	r.cqTail = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.tail]))
	r.cqMask = (*uint32)(unsafe.Pointer(&cqRing[params.cqOff.ringMask]))

	return nil
}

// prepareRead prepares a read operation
func (r *ioURing) prepareRead(fd int, buf []byte, offset int64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get next sqe
	head := *r.sqHead
	next := *r.sqTail
	if (next - head) == r.params.sqEntries {
		return 0, syscall.EBUSY
	}

	index := next & *r.sqMask
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
	sqe.userData = uint64(index)

	// Update array - fix the indexing issue
	arrayPtr := (*[1024]uint32)(unsafe.Pointer(r.sqArray))
	arrayPtr[index] = index
	
	// Update tail
	*r.sqTail = next + 1

	return uint64(index), nil
}

// submit submits the prepared operations
func (r *ioURing) submit() (int, error) {
	return ioUringEnter(r.fd, 1, 1, IORING_ENTER_GETEVENTS)
}

// waitCompletion waits for a completion event
func (r *ioURing) waitCompletion(userData uint64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		head := *r.cqHead
		tail := *r.cqTail

		if head == tail {
			// No completions available, wait
			_, err := ioUringEnter(r.fd, 0, 1, IORING_ENTER_GETEVENTS)
			if err != nil {
				return 0, err
			}
			continue
		}

		// Process completions
		for ; head != tail; head++ {
			index := head & *r.cqMask
			cqeOffset := r.params.cqOff.cqes + index*16
			cqe := (*ioUringCQE)(unsafe.Pointer(&r.cqRing[cqeOffset]))

			if cqe.userData == userData {
				// Found our completion
				if cqe.res < 0 {
					err := syscall.Errno(-cqe.res)
					*r.cqHead = head + 1
					return 0, err
				}

				*r.cqHead = head + 1
				return int(cqe.res), nil
			}
		}

		// Update head
		*r.cqHead = head
	}
}

// close closes the io_uring instance
func (r *ioURing) close() error {
	if r.sqRing != nil {
		unix.Munmap(r.sqRing)
	}
	if r.cqRing != nil {
		unix.Munmap(r.cqRing)
	}
	if r.sqes != nil {
		unix.Munmap(r.sqes)
	}
	if r.fd != 0 {
		unix.Close(r.fd)
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