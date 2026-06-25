package torrent

import "testing"

// TestResolveHashStrategy covers the production strategy-selection logic that
// real `mkbrr create` invocations go through (the equivalence/fuzz tests force
// a strategy and bypass it).
func TestResolveHashStrategy(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("MKBRR_HASH_MMAP", "")
		t.Setenv("MKBRR_MMAP_WINDOW", "")
		s := resolveHashStrategy()
		if s.useMmap != mmapSupported {
			t.Fatalf("default useMmap=%v want %v", s.useMmap, mmapSupported)
		}
		if s.mmapWindow != defaultMmapWindow {
			t.Fatalf("default window=%d want %d", s.mmapWindow, defaultMmapWindow)
		}
	})

	t.Run("disable via env", func(t *testing.T) {
		for _, v := range []string{"0", "false", "off", "no"} {
			t.Setenv("MKBRR_HASH_MMAP", v)
			if resolveHashStrategy().useMmap {
				t.Fatalf("MKBRR_HASH_MMAP=%q should disable mmap", v)
			}
		}
	})

	t.Run("enable via env never exceeds platform support", func(t *testing.T) {
		for _, v := range []string{"1", "true", "on", "yes"} {
			t.Setenv("MKBRR_HASH_MMAP", v)
			if got := resolveHashStrategy().useMmap; got != mmapSupported {
				t.Fatalf("MKBRR_HASH_MMAP=%q useMmap=%v want %v", v, got, mmapSupported)
			}
		}
	})

	t.Run("window override", func(t *testing.T) {
		t.Setenv("MKBRR_HASH_MMAP", "")
		t.Setenv("MKBRR_MMAP_WINDOW", "1048576")
		if got := resolveHashStrategy().mmapWindow; got != 1<<20 {
			t.Fatalf("window=%d want %d", got, 1<<20)
		}
	})

	t.Run("invalid/non-positive window ignored", func(t *testing.T) {
		for _, v := range []string{"notanumber", "0", "-5"} {
			t.Setenv("MKBRR_MMAP_WINDOW", v)
			if got := resolveHashStrategy().mmapWindow; got != defaultMmapWindow {
				t.Fatalf("MKBRR_MMAP_WINDOW=%q window=%d want default %d", v, got, defaultMmapWindow)
			}
		}
	})
}

// TestNewPieceHasherMmapSizeGate verifies the totalSize < minMmapFileSize gate
// in NewPieceHasher disables mmap for tiny inputs and leaves it on (where
// supported) for larger ones.
func TestNewPieceHasherMmapSizeGate(t *testing.T) {
	t.Setenv("MKBRR_HASH_MMAP", "") // default-on where supported

	dir := t.TempDir()
	tiny := writeLayoutFiles(t, dir, []int64{minMmapFileSize - 1})
	if h := NewPieceHasher(tiny, 1<<14, 1, &mockDisplay{}, false); h.strategy.useMmap {
		t.Fatalf("tiny input (<%d) should not use mmap", minMmapFileSize)
	}

	big := writeLayoutFiles(t, dir, []int64{minMmapFileSize + 4096})
	total := minMmapFileSize + 4096
	numPieces := int((total + (1 << 14) - 1) / (1 << 14))
	if h := NewPieceHasher(big, 1<<14, numPieces, &mockDisplay{}, false); h.strategy.useMmap != mmapSupported {
		t.Fatalf("large input useMmap=%v want %v", h.strategy.useMmap, mmapSupported)
	}
}
