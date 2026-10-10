//go:build test_unit && windows

package output

// Windows counterpart to driver-pipe_unix_test.go (mutually exclusive by
// build tag, so no symbol clash) -- exercises the TCP loopback dial
// driver-pipe-windows.go uses instead of a POSIX FIFO.

import (
	"io"
	"net"
	"testing"
	"time"
)

// eofReader is a Float32Reader that reports end of stream, so the pipe
// output loop goes idle and blocks on the cond var instead of writing.
type eofReader struct{}

func (eofReader) Read([]float32) (int, error) {
	return 0, io.EOF
}

func pipeOutputOptions(addr string) *NewOutputOptions {
	return &NewOutputOptions{
		Reader:           eofReader{},
		OutputPipe:       addr,
		OutputPipeFormat: "s16le",
	}
}

func TestPipeOutputConnectsToListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	out, err := newPipeOutput(pipeOutputOptions(ln.Addr().String()))
	if err != nil {
		t.Fatalf("newPipeOutput returned error: %v", err)
	}
	defer out.Close()

	select {
	case conn := <-accepted:
		conn.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the pipe output to connect to the listener")
	}
}

func TestPipeOutputFailsWithoutListener(t *testing.T) {
	// Port 1 on loopback: nothing listens there, and a refused TCP
	// connection on localhost comes back near-instantly, well under
	// pipeDialTimeout, so this doesn't need to wait out the full timeout.
	_, err := newPipeOutput(pipeOutputOptions("127.0.0.1:1"))
	if err == nil {
		t.Fatal("expected an error connecting with no listener")
	}
}

func TestPipeOutputRejectsUnknownFormat(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer ln.Close()

	_, err = newPipeOutput(&NewOutputOptions{
		Reader:           eofReader{},
		OutputPipe:       ln.Addr().String(),
		OutputPipeFormat: "bogus",
	})
	if err == nil {
		t.Fatal("expected an error for an unknown pipe format")
	}
}
