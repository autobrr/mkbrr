package torrent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeLayoutFiles writes files whose sizes are given by sizes, with
// deterministic-but-varied content, and returns the fileEntry layout.
func writeLayoutFiles(tb testing.TB, dir string, sizes []int64) []fileEntry {
	tb.Helper()
	files := make([]fileEntry, 0, len(sizes))
	var offset int64
	for i, sz := range sizes {
		p := filepath.Join(dir, fmt.Sprintf("f%03d.bin", i))
		buf := make([]byte, sz)
		// content varies by file index and position so a mis-mapped offset or
		// a dropped/duplicated byte changes the hash.
		for j := range buf {
			buf[j] = byte((j*31 + i*7 + 11) % 251)
		}
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			tb.Fatalf("write %s: %v", p, err)
		}
		files = append(files, fileEntry{path: p, length: sz, offset: offset})
		offset += sz
	}
	return files
}

// hashWith runs the piece hasher over files with the given strategy and returns
// the concatenated piece hashes.
func hashWith(tb testing.TB, files []fileEntry, pieceLen int64, workers int, st hashStrategy) []byte {
	tb.Helper()
	var total int64
	for _, f := range files {
		total += f.length
	}
	numPieces := 0
	if pieceLen > 0 {
		numPieces = int((total + pieceLen - 1) / pieceLen)
	}
	h := NewPieceHasher(files, pieceLen, numPieces, &mockDisplay{}, false)
	h.strategy = st // force the path under test, bypassing env/size gates
	if err := h.hashPieces(workers); err != nil {
		tb.Fatalf("hashPieces(%+v): %v", st, err)
	}
	out := make([]byte, 0, numPieces*20)
	for _, p := range h.pieces {
		out = append(out, p...)
	}
	return out
}

// TestChunkReaderEquivalence asserts the mmap path produces byte-identical
// piece hashes to the read() path across many file/piece layouts and window
// sizes. Byte-identical output is the whole correctness contract: a torrent's
// piece hashes must not depend on how the bytes were read.
func TestChunkReaderEquivalence(t *testing.T) {
	if !mmapSupported {
		t.Skip("mmap not supported on this platform")
	}

	const pageSize = 4096
	layouts := []struct {
		name  string
		sizes []int64
	}{
		{"single-exact", []int64{8 * pageSize}},
		{"single-tiny", []int64{100}},
		{"single-odd", []int64{8*pageSize + 123}},
		{"single-sub-piece", []int64{500}},
		{"two-files-span", []int64{pageSize + 17, 5 * pageSize}},
		{"many-small", []int64{1, 2, 3, 4, 5, 100, 200, 4097, 1, 9999}},
		{"empty-then-data", []int64{0, 4096, 0, 12345}},
		{"big-and-small", []int64{1 << 20, 7, 1<<20 + 3}},
		{"piece-spans-three", []int64{pageSize / 2, pageSize / 2, pageSize / 2, pageSize/2 + 1}},
	}
	pieceLens := []int64{1 << 14, pageSize, 3000, 7 * pageSize}
	windows := []int64{pageSize, 64 * 1024, 1 << 20}
	workerCounts := []int{1, 4}

	for _, lay := range layouts {
		for _, pl := range pieceLens {
			for _, workers := range workerCounts {
				dir := t.TempDir()
				files := writeLayoutFiles(t, dir, lay.sizes)
				want := hashWith(t, files, pl, workers, hashStrategy{useMmap: false})
				for _, win := range windows {
					got := hashWith(t, files, pl, workers, hashStrategy{useMmap: true, mmapWindow: win})
					if !bytes.Equal(want, got) {
						t.Fatalf("mismatch layout=%s piece=%d workers=%d window=%d\n read=%x\n mmap=%x",
							lay.name, pl, workers, win, want, got)
					}
				}
			}
		}
	}
}

// FuzzChunkReaderEquivalence exhaustively asserts read() and mmap agree for
// arbitrary file partitions, piece lengths, and window sizes.
func FuzzChunkReaderEquivalence(f *testing.F) {
	if !mmapSupported {
		f.Skip("mmap not supported on this platform")
	}
	f.Add([]byte("hello world this is some torrent data"), uint16(7), uint8(3), uint16(4096))
	f.Add(bytes.Repeat([]byte{0xAB, 0x00, 0x01}, 5000), uint16(4096), uint8(1), uint16(13))
	f.Add([]byte{}, uint16(1), uint8(1), uint16(1))

	f.Fuzz(func(t *testing.T, data []byte, pieceSeed uint16, nfilesSeed uint8, winSeed uint16) {
		nfiles := int(nfilesSeed)%6 + 1
		pieceLen := int64(pieceSeed)%8192 + 1
		window := int64(winSeed) + 1

		// partition data into nfiles roughly-equal files
		sizes := make([]int64, nfiles)
		base := int64(len(data)) / int64(nfiles)
		var assigned int64
		for i := 0; i < nfiles; i++ {
			if i == nfiles-1 {
				sizes[i] = int64(len(data)) - assigned
			} else {
				sizes[i] = base
			}
			assigned += sizes[i]
		}

		dir := t.TempDir()
		files := make([]fileEntry, 0, nfiles)
		var off int64
		for i, sz := range sizes {
			p := filepath.Join(dir, fmt.Sprintf("f%d.bin", i))
			if err := os.WriteFile(p, data[off:off+sz], 0o644); err != nil {
				t.Fatal(err)
			}
			files = append(files, fileEntry{path: p, length: sz, offset: off})
			off += sz
		}

		want := hashWith(t, files, pieceLen, 1, hashStrategy{useMmap: false})
		got := hashWith(t, files, pieceLen, 1, hashStrategy{useMmap: true, mmapWindow: window})
		if !bytes.Equal(want, got) {
			t.Fatalf("mismatch nfiles=%d piece=%d window=%d len=%d\n read=%x\n mmap=%x",
				nfiles, pieceLen, window, len(data), want, got)
		}
	})
}
