package compressor_test

import (
	"io"
	"testing"

	"github.com/onflow/flow-go/network/compressor"
)

func TestCorrectness(t *testing.T) {
	text := "hello world!"

	pr, pw := io.Pipe()

	gzipComp := compressor.GzipStreamCompressor{}
	gzipComp.NewWriter(&s)
}
