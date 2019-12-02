package jsonrpc2

import "os"

// stdrwc implements an io.ReadWriter and io.Closer around STDIN and STDOUT.
type stdrwc struct{}

// Read reads from STDIN.
func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

// Write writes to STDOUT.
func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close closes STDIN and STDOUT.
func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
