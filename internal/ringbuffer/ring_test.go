package ringbuffer

import (
	"bytes"
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

	// Channels to signal completion of writer and reader
	writerDone := make(chan struct{})
	readerDone := make(chan struct{})

	// Writer goroutine
	go func() {
		defer close(writerDone)
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))} // Cycle through 'a' to 'z'
			_, err := rb.Write(data)
			if err != nil {
				t.Errorf("Write error: %v", err)
				return
			}
		}
	}()

	// Reader goroutine
	go func() {
		defer close(readerDone)
		buf := make([]byte, 1)
		for i := 0; i < 100; i++ {
			_, err := rb.Read(buf)
			if err != nil && err != io.EOF {
				t.Errorf("Read error: %v", err)
				return
			}
		}
	}()

	// Wait for both goroutines to finish
	<-writerDone
	<-readerDone
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

	// Channels to signal completion of writer and reader
	writerDone := make(chan struct{})
	readerDone := make(chan struct{})

	// Write data
	go func() {
		defer close(writerDone)
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))} // Cycle through 'a' to 'z'
			_, err := rb.Write(data)
			if err != nil {
				t.Errorf("Write error: %v", err)
				return
			}
		}
	}()

	// Read data
	go func() {
		defer close(readerDone)
		buf := make([]byte, 1)
		for i := 0; i < 100; i++ {
			data := []byte{byte('a' + (i % 26))}
			_, err := rb.Read(buf)
			if err != nil {
				t.Errorf("Read error: %v", err)
				return
			}
			if data[0] != buf[0] {
				t.Errorf("bad bytes %d %c/%c", i, data[0], buf[0])
			}
		}
	}()

	// Wait for both goroutines to complete
	<-writerDone
	<-readerDone
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

	// Writer goroutine to write data in chunks
	go func() {
		chunks := []string{"hello", "world", "this", "is", "test"}
		for _, chunk := range chunks {
			_, err := rb.Write([]byte(chunk))
			if err != nil {
				t.Errorf("Write error: %v", err)
			}
			time.Sleep(1 * time.Millisecond) // Simulate delay between writes
		}
		rb.CloseWriter() // Signal end of writing
	}()

	// Reader to read data into the large buffer
	b := &bytes.Buffer{}
	n, err := io.Copy(b, rb)
	if err != io.EOF && err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != int64(len(expected)) || string(b.Bytes()[:n]) != expected {
		t.Errorf("Expected to read '%s', got '%s'", expected, string(b.Bytes()[:n]))
	}
}

func TestRingBuffer_ConcurrentWorkers(t *testing.T) {
	rb := New(20) // Buffer size of 20

	// Number of workers
	numWriters := 50
	numReaders := 50
	numIterations := 50

	// Channels to signal completion of workers
	writerDone := make(chan struct{}, numWriters)
	readerDone := make(chan struct{}, numReaders)

	// Writer workers
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer func() { writerDone <- struct{}{} }()
			for j := 0; j < numIterations; j++ {
				data := []byte{byte('A' + (id % 26))} // Each writer writes its own character
				_, err := rb.Write(data)
				if err != nil {
					t.Errorf("Writer %d error: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Reader workers
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer func() { readerDone <- struct{}{} }()
			buf := make([]byte, 1)
			for j := 0; j < numIterations; j++ {
				_, err := rb.Read(buf)
				if err != nil && err != io.EOF {
					t.Errorf("Reader %d error: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	for i := 0; i < numWriters; i++ {
		<-writerDone
	}
	for i := 0; i < numReaders; i++ {
		<-readerDone
	}

	// Ensure the buffer is empty after all workers are done
	if !rb.isEmpty() {
		t.Errorf("Buffer should be empty after all workers are done")
	}
}
