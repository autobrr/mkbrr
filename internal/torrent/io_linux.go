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
	IORING_OP_READ = 22  // Correct value is 22, not 1

	// Flags
	IORING_ENTER_GETEVENTS = 1
	
	// Offsets
	IORING_OFF_SQ_RING = 0
	IORING_OFF_CQ_RING = 0x8000000
	IORING_OFF_SQES    = 0x10000000
	
	// Max attempts to wait for completion before falling back
	maxCompletionAttempts = 1000
)

// ioURingConfig allows adjusting io_uring behavior
var ioURingConfig struct {
	// Disable io_uring completely (can be set by environment or tests)
	Disabled atomic.Bool
	// Debug mode for additional logging/verification
	Debug bool
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
	sqOff        sqRingOffsets
	cqOff        cqRingOffsets
}

type sqRingOffsets struct {
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

type cqRingOffsets struct {
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

// ioUringReader implements ioReader using io_uring
type ioUringReader struct {
	fd     int
	path   string
	closed bool
	ring   *ioURing
}

// ioURing represents an io_uring instance
type ioURing struct {
	fd           int
	params       ioUringParams
	sqRing       []byte
	cqRing       []byte
	sqes         []byte
	sqHead       *uint32
	sqTail       *uint32
	sqMask       *uint32
	sqArray      *uint32
	cqHead       *uint32
	cqTail       *uint32
	cqMask       *uint32
	mu           sync.Mutex
	nextUserData uint64
}

// Global io_uring instance and control
var (
	globalRing     *ioURing
	globalRingOnce sync.Once
	ringSupported  bool
)

// DisableIoURing disables io_uring for testing or compatibility
func DisableIoURing() {
	ioURingConfig.Disabled.Store(true)
}

// EnableIoURing enables io_uring if supported
func EnableIoURing() {
	ioURingConfig.Disabled.Store(false)
}

// ioUringSetup calls the io_uring_setup syscall
func ioUringSetup(entries uint32, params *ioUringParams) (int, error) {
	// io_uring_setup is syscall 425 on most platforms
	r1, _, errno := syscall.Syscall(425, uintptr(entries), uintptr(unsafe.Pointer(params)), 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// ioUringEnter calls the io_uring_enter syscall
func ioUringEnter(fd int, toSubmit uint32, minComplete uint32, flags uint32) (int, error) {
	// io_uring_enter is syscall 426 on most platforms
	r1, _, errno := syscall.Syscall6(426, uintptr(fd), uintptr(toSubmit), uintptr(minComplete), uintptr(flags), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// getGlobalRing returns a shared io_uring instance
func getGlobalRing() (*ioURing, error) {
	if ioURingConfig.Disabled.Load() {
		return nil, syscall.ENOSYS
	}

	globalRingOnce.Do(func() {
		ring := &ioURing{}
		err := ring.init(128) // 128 entries is a good default
		if err == nil {
			globalRing = ring
			ringSupported = true
		} else {
			// If we failed to initialize, disable for future calls
			ringSupported = false
			ioURingConfig.Disabled.Store(true)
		}
	})
	
	if !ringSupported || ioURingConfig.Disabled.Load() {
		return nil, syscall.ENOSYS
	}
	return globalRing, nil
}

// init initializes an io_uring instance
func (r *ioURing) init(entries uint32) error {
	// Set up parameters
	params := ioUringParams{}
	fd, err := ioUringSetup(entries, &params)
	if err != nil {
		return err
	}
	r.fd = fd
	r.params = params

	// Map the submission queue ring buffer
	sqRingSize := params.sqOff.array + params.sqEntries*4
	sqRing, err := unix.Mmap(fd, IORING_OFF_SQ_RING, int(sqRingSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(fd)
		return err
	}
	r.sqRing = sqRing

	// Map the completion queue ring buffer
	cqRingSize := params.cqOff.cqes + params.cqEntries*16
	cqRing, err := unix.Mmap(fd, IORING_OFF_CQ_RING, int(cqRingSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqRing)
		unix.Close(fd)
		return err
	}
	r.cqRing = cqRing

	// Map the submission queue entries
	sqesSize := params.sqEntries * 64 // Each SQE is 64 bytes
	sqes, err := unix.Mmap(fd, IORING_OFF_SQES, int(sqesSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(cqRing)
		unix.Munmap(sqRing)
		unix.Close(fd)
		return err
	}
	r.sqes = sqes

	// Set up pointers to ring elements
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

	// Check if there's space in the submission queue
	head := atomic.LoadUint32(r.sqHead)
	next := atomic.LoadUint32(r.sqTail)
	
	if next-head >= r.params.sqEntries {
		return 0, syscall.EBUSY
	}

	// Get index in the ring
	index := next & *r.sqMask
	
	// Get pointer to the SQE at this index
	sqeOffset := index * 64
	sqe := (*ioUringSQE)(unsafe.Pointer(&r.sqes[sqeOffset]))
	
	// Clear the SQE first to avoid any lingering values
	*sqe = ioUringSQE{}
	
	// Set up the read operation
	sqe.opcode = IORING_OP_READ
	sqe.fd = int32(fd)
	sqe.addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
	sqe.len = uint32(len(buf))
	sqe.off = uint64(offset)
	
	// Generate unique user data
	r.nextUserData++
	userData := r.nextUserData
	sqe.userData = userData
	
	// Update the array - this is how the kernel knows which SQE to use
	arrayPtr := (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(r.sqArray)) + uintptr(index*4)))
	*arrayPtr = index
	
	// Increment the tail pointer to indicate we've added an entry
	atomic.AddUint32(r.sqTail, 1)
	
	return userData, nil
}

// submit submits operations to the kernel
func (r *ioURing) submit() (int, error) {
	return ioUringEnter(r.fd, 1, 1, IORING_ENTER_GETEVENTS)
}

// waitCompletion waits for a completion event with retry limit
func (r *ioURing) waitCompletion(userData uint64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Maximum number of attempts to prevent hanging
	for attempts := 0; attempts < maxCompletionAttempts; attempts++ {
		// Check if there are any completions
		head := atomic.LoadUint32(r.cqHead)
		tail := atomic.LoadUint32(r.cqTail)
		
		if head != tail {
			// Process completion queue entries
			for ; head != tail; head++ {
				index := head & *r.cqMask
				cqeOffset := r.params.cqOff.cqes + index*16
				cqe := (*ioUringCQE)(unsafe.Pointer(&r.cqRing[cqeOffset]))
				
				if cqe.userData == userData {
					// Found our completion
					res := cqe.res
					
					// Update head pointer
					atomic.StoreUint32(r.cqHead, head+1)
					
					if res < 0 {
						return 0, syscall.Errno(-res)
					}
					
					return int(res), nil
				}
			}
			
			// Update head pointer if we processed entries but didn't find ours
			atomic.StoreUint32(r.cqHead, head)
		}
		
		// Wait for more completions
		_, err := ioUringEnter(r.fd, 0, 1, IORING_ENTER_GETEVENTS)
		if err != nil {
			// If any error occurs here, disable io_uring for future operations
			ioURingConfig.Disabled.Store(true)
			return 0, err
		}
	}
	
	// If we reach here, we've hit the maximum number of attempts
	// Disable io_uring for future operations and return timeout
	ioURingConfig.Disabled.Store(true)
	return 0, syscall.ETIMEDOUT
}

// close closes and cleans up the io_uring instance
func (r *ioURing) close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	var err error
	
	if r.sqes != nil {
		err = unix.Munmap(r.sqes)
		r.sqes = nil
	}
	
	if r.cqRing != nil {
		err = unix.Munmap(r.cqRing)
		r.cqRing = nil
	}
	
	if r.sqRing != nil {
		err = unix.Munmap(r.sqRing)
		r.sqRing = nil
	}
	
	if r.fd != 0 {
		err = unix.Close(r.fd)
		r.fd = 0
	}
	
	return err
}

// NewIOReader creates an appropriate IO reader for a file
func NewIOReader(path string) (ioReader, error) {
	// Try to use io_uring if available and not disabled
	if !ioURingConfig.Disabled.Load() {
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
	
	// Fall back to direct I/O if io_uring isn't available
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECT, 0)
	if err == nil {
		return &directReader{file: f}, nil
	}
	
	// Last resort: standard file I/O
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

	// Try io_uring read
	userData, err := r.ring.prepareRead(r.fd, buf, offset)
	if err != nil {
		// Disable io_uring and fall back to pread
		ioURingConfig.Disabled.Store(true)
		return unix.Pread(r.fd, buf, offset)
	}
	
	// Submit operation
	_, err = r.ring.submit()
	if err != nil {
		ioURingConfig.Disabled.Store(true)
		return unix.Pread(r.fd, buf, offset)
	}
	
	// Wait for completion
	n, err := r.ring.waitCompletion(userData)
	if err != nil {
		ioURingConfig.Disabled.Store(true)
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
	
	_, err := r.file.Seek(offset, 0)
	if err != nil {
		return 0, err
	}
	return r.file.Read(buf)
}

// Close closes the file
func (r *stdReader) Close() error {
	return r.file.Close()
} 