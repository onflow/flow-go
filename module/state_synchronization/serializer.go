package state_synchronization

import (
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/network"
)

// header codes to distinguish between different types of data
const (
	CodeRecursiveCIDs = iota
	CodeExecutionData
)

func getCode(v interface{}) byte {
	switch v.(type) {
	case *ExecutionData:
		return CodeExecutionData
	case []cid.Cid:
		return CodeRecursiveCIDs
	default:
		panic(fmt.Sprintf("invalid type for interface: %T", v))
	}
}

func getPrototype(code byte) interface{} {
	switch code {
	case CodeExecutionData:
		return &ExecutionData{}
	case CodeRecursiveCIDs:
		return &[]cid.Cid{}
	default:
		panic(fmt.Sprintf("invalid code: %v", code))
	}
}

type reader interface {
	io.ByteReader
	io.Reader
}

type writer interface {
	io.ByteWriter
	io.Writer
}

// serializer encapulates the serialization and deserialization of data
type serializer struct {
	codec      encoding.Codec
	compressor network.Compressor
}

func (s *serializer) serialize(w writer, v interface{}) error {
	if err := w.WriteByte(getCode(v)); err != nil {
		return fmt.Errorf("failed to write code: %w", err)
	}

	comp, err := s.compressor.NewWriter(w)
	if err != nil {
		return fmt.Errorf("failed to create compressor writer: %w", err)
	}

	enc := s.codec.NewEncoder(comp)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}
	if err := comp.Close(); err != nil {
		return fmt.Errorf("failed to close compressor: %w", err)
	}

	return nil
}

func (s *serializer) deserialize(r reader) (interface{}, error) {
	code, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read code: %w", err)
	}

	comp, err := s.compressor.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor reader: %w", err)
	}

	dec := s.codec.NewDecoder(comp)

	v := getPrototype(code)
	if err := dec.Decode(v); err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return v, nil
}
