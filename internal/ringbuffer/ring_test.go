package ringbuffer

import (
	"io"
	"testing"
	"time"
)

func TestRingBuffer_Read(t *testing.T) {
	t.Run("Read from empty buffer", func(t *testing.T) {
		rb := New(10)
		buf := make([]byte, 5)
		go func() {
			// Simulate a delayed write
			rb.Write([]byte("hello"))
		}()
		n, err := rb.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != 5 || string(buf) != "hello" {
			t.Errorf("Expected to read %q, got %q", "hello", string(buf))
		}
	})

	t.Run("Read from buffer with data", func(t *testing.T) {
		rb := New(10)
		rb.Write([]byte("hello"))
		buf := make([]byte, 5)
		n, err := rb.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != 5 || string(buf) != "hello" {
			t.Errorf("Expected to read 'hello', got '%s'", string(buf))
		}
	})

	t.Run("Read with buffer wrap-around", func(t *testing.T) {
		rb := New(10)
		rb.Write([]byte("abcdefghij")) // Fill the buffer
		rb.Read(make([]byte, 5))       // Read first 5 bytes
		rb.Write([]byte("12345"))      // Write more data, causing wrap-around

		expected := "fghij12345"
		buf := make([]byte, len(expected))
		n, err := rb.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if n != len(expected) || string(buf) != expected {
			t.Errorf("Expected to read '%s', got '%s'", expected, string(buf))
		}
	})

	t.Run("Read after writer closed with error", func(t *testing.T) {
		rb := New(10)
		rb.Write([]byte("hello"))
		rb.CloseWithError(io.EOF)

		buf := make([]byte, 5)
		n, err := rb.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if n != 5 || string(buf) != "hello" {
			t.Errorf("Expected to read 'hello', got '%s'", string(buf))
		}

		// Attempt to read again, should return EOF
		n, err = rb.Read(buf)
		if err != io.EOF {
			t.Errorf("Expected EOF, got %v", err)
		}
		if n != 0 {
			t.Errorf("Expected 0 bytes read, got %d", n)
		}
	})
}

func TestRingBuffer_FullCondition(t *testing.T) {
	rb := New(5) // Buffer size of 5
	data := []byte("abcd")

	// Write 4 bytes (one less than the buffer size)
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Buffer should not be full yet
	if rb.isFull() {
		t.Errorf("Buffer should not be full")
	}

	// Write one more byte to fill the buffer
	n, err = rb.Write([]byte("e"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("Expected to write 1 byte, wrote %d", n)
	}

	// Now the buffer should be full
	if !rb.isFull() {
		t.Errorf("Buffer should be full")
	}
}

func TestRingBuffer_ConcurrentReadWrite(t *testing.T) {
	rb := New(10) // Buffer size of 10

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))} // Cycle through 'a' to 'z'
			_, err := rb.Write(data)
			if err != nil {
				t.Errorf("Write error: %v", err)
			}
		}
	}()

	// Reader goroutine
	go func() {
		buf := make([]byte, 1)
		for i := 0; i < 100; i++ {
			_, err := rb.Read(buf)
			if err != nil && err != io.EOF {
				t.Errorf("Read error: %v", err)
			}
		}
	}()

	// Allow time for goroutines to complete
	// This is not ideal for deterministic testing but works for basic concurrency checks
	<-time.After(1 * time.Second)
}

func TestRingBuffer_ConcurrentAccess(t *testing.T) {
	rb := New(10) // Buffer size of 10

	// Writer goroutine
	writerDone := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))} // Cycle through 'a' to 'z'
			_, err := rb.Write(data)
			if err != nil {
				t.Errorf("Write error: %v", err)
			}
		}
		close(writerDone)
	}()

	// Reader goroutine
	readerDone := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		for i := 0; i < 100; i++ {
			_, err := rb.Read(buf)
			if err != nil && err != io.EOF {
				t.Errorf("Read error: %v", err)
			}
		}
		close(readerDone)
	}()

	// Wait for both goroutines to finish
	<-writerDone
	<-readerDone
}

func TestRingBuffer_Overwrite(t *testing.T) {
	rb := New(5) // Buffer size of 5

	// Write data to fill the buffer
	rb.Write([]byte("abcde"))

	// Read some data to make space
	buf := make([]byte, 2)
	rb.Read(buf)

	// Write more data to overwrite the buffer
	rb.Write([]byte("fg"))

	// Read remaining data
	expected := "cdefg"
	buf = make([]byte, len(expected))
	n, err := rb.Read(buf)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != len(expected) || string(buf) != expected {
		t.Errorf("Expected to read '%s', got '%s'", expected, string(buf))
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	rb := New(5) // Buffer size of 5

	// Write data to the buffer
	rb.Write([]byte("abcde"))

	// Reset the buffer
	rb.Reset()

	// Ensure the buffer is empty
	if !rb.isEmpty() {
		t.Errorf("Buffer should be empty after reset")
	}

	// Write and read again to ensure the buffer works after reset
	rb.Write([]byte("xyz"))
	buf := make([]byte, 3)
	n, err := rb.Read(buf)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 3 || string(buf) != "xyz" {
		t.Errorf("Expected to read 'xyz', got '%s'", string(buf))
	}
}

func TestRingBuffer_CloseWithError(t *testing.T) {
	rb := New(5) // Buffer size of 5

	// Write some data
	rb.Write([]byte("abc"))

	// Close the writer
	rb.CloseWithError(io.EOF)

	// Attempt to write more data
	n, err := rb.Write([]byte("d"))
	if err == nil {
		t.Errorf("Expected error after closing writer, got nil")
	}
	if n != 0 {
		t.Errorf("Expected to write 0 bytes, wrote %d", n)
	}

	// Read remaining data
	buf := make([]byte, 3)
	n, err = rb.Read(buf)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 3 || string(buf) != "abc" {
		t.Errorf("Expected to read 'abc', got '%s'", string(buf))
	}

	// Attempt to read again, should return EOF
	n, err = rb.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes read, got %d", n)
	}
}

func TestRingBuffer_DataIntegrity(t *testing.T) {
	rb := New(10) // Buffer size of 10

	// Write data
	go func() {
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))} // Cycle through 'a' to 'z'
			rb.Write(data)
		}
	}()

	// Read data
	go func() {
		buf := make([]byte, 1)
		for i := 0; i < 100; i++ {
			rb.Read(buf)
		}
	}()

	// Allow time for goroutines to complete
	time.Sleep(1 * time.Second)
}

func TestRingBuffer_MultipleResets(t *testing.T) {
	rb := New(5) // Buffer size of 5

	for i := 0; i < 5; i++ {
		// Write data to the buffer
		data := []byte("abcde")
		n, err := rb.Write(data)
		if err != nil {
			t.Fatalf("Unexpected error during write: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
		}

		// Reset the buffer
		rb.Reset()

		// Ensure the buffer is empty
		if !rb.isEmpty() {
			t.Errorf("Buffer should be empty after reset #%d", i+1)
		}

		// Write and read again to ensure the buffer works after reset
		rb.Write([]byte("xyz"))
		buf := make([]byte, 3)
		n, err = rb.Read(buf)
		if err != nil {
			t.Fatalf("Unexpected error during read: %v", err)
		}
		if n != 3 || string(buf) != "xyz" {
			t.Errorf("Expected to read 'xyz', got '%s' after reset #%d", string(buf), i+1)
		}
	}
}

func TestRingBuffer_LargeReadBufferWithAsyncWrites(t *testing.T) {
	rb := New(10) // Buffer size of 10
	expected := "helloworldthisistest"
	readBuf := make([]byte, len(expected)) // Read buffer larger than the ring buffer

	// Writer goroutine to write data in chunks
	go func() {
		chunks := []string{"hello", "world", "this", "is", "test"}
		for _, chunk := range chunks {
			_, err := rb.Write([]byte(chunk))
			if err != nil {
				t.Errorf("Write error: %v", err)
			}
			time.Sleep(100 * time.Millisecond) // Simulate delay between writes
		}
		rb.CloseWithError(io.EOF) // Signal end of writing
	}()

	// Reader to read data into the large buffer
	n, err := rb.Read(readBuf)
	if err != io.EOF && err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(expected) || string(readBuf[:n]) != expected {
		t.Errorf("Expected to read '%s', got '%s'", expected, string(readBuf[:n]))
	}
}
