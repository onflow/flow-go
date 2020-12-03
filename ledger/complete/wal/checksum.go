package wal

import (
	"fmt"
	"hash"
	"hash/crc32"
	"io"
)

var crc32Table = crc32.MakeTable(crc32.Castagnoli)

type Crc32Writer struct {
	hash   hash.Hash32
	Writer io.Writer
}

func NewCRC32Writer(writer io.Writer) *Crc32Writer {
	return &Crc32Writer{
		hash:   crc32.New(crc32Table),
		Writer: writer,
	}
}

func (c *Crc32Writer) Write(p []byte) (n int, err error) {

	// hash.Write never fails, but who knows
	n, err = c.hash.Write(p)
	if err != nil {
		return n, fmt.Errorf("error while calculating crc32: %w", err)
	}

	return c.Writer.Write(p)
}

func (c *Crc32Writer) Crc32() uint32 {
	return c.hash.Sum32()
}

type Crc32Reader struct {
	hash   hash.Hash32
	reader io.Reader
}

func NewCRC32Reader(reader io.Reader) *Crc32Reader {
	return &Crc32Reader{
		hash:   crc32.New(crc32Table),
		reader: reader,
	}
}

func (c *Crc32Reader) Read(p []byte) (int, error) {

	read, err := c.reader.Read(p)
	if err != nil {
		return read, fmt.Errorf("error while reading for crc32 sum: %w", err)
	}
	_, err = c.hash.Write(p[:read])
	if err != nil {
		return 0, fmt.Errorf("error while calculating crc32: %w", err)
	}

	return read, err
}

func (c *Crc32Reader) Crc32() uint32 {
	return c.hash.Sum32()
}
