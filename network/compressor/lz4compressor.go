package compressor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pierrec/lz4"
)

type Lz4Compressor struct {
	reader *lz4.Reader
	writer *lz4.Writer
}

func NewLz4Compressor() *Lz4Compressor {
	return &Lz4Compressor{}
}

func Compress(payload []byte) ([]byte, error) {
	b := bytes.NewBuffer(payload)

	pr, pw := io.Pipe()

	zw := lz4.NewWriter(pw)
	defer zw.Close()

	zr := lz4.NewReader(pr)
	defer pw.Close()

	_, _ = io.Copy(zw, b)

	compressed, err := ioutil.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("could not read from pipe: %w", err)
	}

	return compressed, nil
}

func Decompress(payload []byte) ([]byte, error) {
	return nil, nil
}
