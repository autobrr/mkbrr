//go:build unix

package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// TestMmapShortFileErrors guards the silent-zero-fill regression: a file shorter
// on disk than its recorded fileEntry.length (e.g. shrunk/written between scan
// and hashing) must produce an error from the mmap path — matching the read()
// path's io.ErrUnexpectedEOF — not a hash computed over the kernel's zero-fill.
// The dangerous case is a sub-page shortfall, which never touches a fully
// unbacked page and so fires no SIGBUS.
func TestMmapShortFileErrors(t *testing.T) {
	if !mmapSupported {
		t.Skip("mmap unsupported on this platform")
	}
	ps := int64(unix.Getpagesize())
	actual := ps/2 + 123
	recorded := actual + 400 // claim more than exists; shortfall stays in the last backed page
	if recorded > ps {
		t.Skipf("page size %d too small for sub-page shortfall test", ps)
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "short.bin")
	content := make([]byte, actual)
	for i := range content {
		content[i] = byte(i%250 + 1) // never zero, so zero-fill is detectable
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}

	open := func() *fileReader {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		return &fileReader{file: f, length: recorded}
	}

	frM := open()
	defer func() { _ = frM.file.Close() }()
	errM := newMmapChunkReader(defaultMmapWindow).feed(sha1.New(), frM, 0, recorded)

	frB := open()
	defer func() { _ = frB.file.Close() }()
	bbuf := make([]byte, 1<<20)
	errB := (&bufferedChunkReader{buf: &bbuf}).feed(sha1.New(), frB, 0, recorded)

	if errB == nil {
		t.Fatal("read() path unexpectedly succeeded on a short file; test cannot establish the baseline")
	}
	if errM == nil {
		t.Fatalf("mmap path silently accepted a short file (would bake a wrong hash); read() path errored with %v", errB)
	}
}

// TestMmapFallbackMatchesRead forces every mmap syscall to fail and asserts the
// read() fallback inside mmapChunkReader produces byte-identical piece hashes to
// the pure read() path — i.e. mkbrr still hashes correctly on filesystems where
// mmap is unavailable instead of aborting.
func TestMmapFallbackMatchesRead(t *testing.T) {
	if !mmapSupported {
		t.Skip("mmap unsupported on this platform")
	}
	orig := mmapFunc
	mmapFunc = func(fd int, offset int64, length, prot, flags int) ([]byte, error) {
		return nil, unix.ENODEV
	}
	defer func() { mmapFunc = orig }()

	dir := t.TempDir()
	files := writeLayoutFiles(t, dir, []int64{1, 5000, 1<<20 + 7, 0, 123456})
	for _, pl := range []int64{1 << 14, 4096, 7000} {
		want := hashWith(t, files, pl, 1, hashStrategy{useMmap: false})
		got := hashWith(t, files, pl, 1, hashStrategy{useMmap: true, mmapWindow: defaultMmapWindow})
		if !bytes.Equal(want, got) {
			t.Fatalf("mmap read-fallback mismatch piece=%d\n read=%x\n fallback=%x", pl, want, got)
		}
	}
}
