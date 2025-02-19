package sha1

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"testing"
)

func TestSHA1(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq", "84983e441c3bd26ebaae4aa1f95129e5e54670f1"},
		{
			"abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu",
			"a49b2446a02c645bf419f995b67091253a04a259",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test-%d", i), func(t *testing.T) {
			// Test our implementation
			h := New()
			h.Write([]byte(tt.input))
			got := fmt.Sprintf("%x", h.Sum(nil))
			if got != tt.want {
				t.Errorf("SHA1(%q) = %q, want %q", tt.input, got, tt.want)
			}

			// Compare with standard library
			h2 := sha1.New()
			h2.Write([]byte(tt.input))
			want := fmt.Sprintf("%x", h2.Sum(nil))
			if got != want {
				t.Errorf("SHA1(%q) = %q, want %q (standard library)", tt.input, got, want)
			}
		})
	}
}

func TestSHA1Long(t *testing.T) {
	// Test with a long input to ensure block processing works correctly
	input := bytes.Repeat([]byte("a"), 1000000)

	// Test our implementation
	h := New()
	h.Write(input)
	got := fmt.Sprintf("%x", h.Sum(nil))

	// Compare with standard library
	h2 := sha1.New()
	h2.Write(input)
	want := fmt.Sprintf("%x", h2.Sum(nil))

	if got != want {
		t.Errorf("SHA1(1M 'a's) = %q, want %q", got, want)
	}
}

func BenchmarkSHA1(b *testing.B) {
	sizes := []int{64, 1024, 8192, 1048576} // 64B, 1KB, 8KB, 1MB

	for _, size := range sizes {
		input := bytes.Repeat([]byte("a"), size)

		b.Run(fmt.Sprintf("hardware-%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				h := New()
				h.Write(input)
				h.Sum(nil)
			}
		})

		b.Run(fmt.Sprintf("standard-%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				h := sha1.New()
				h.Write(input)
				h.Sum(nil)
			}
		})
	}
}
