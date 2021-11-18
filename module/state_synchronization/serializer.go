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

func getCode(v interface{}) (byte, error) {
	switch v.(type) {
	case *ExecutionData:
		return CodeExecutionData, nil
	case []cid.Cid:
		return CodeRecursiveCIDs, nil
	default:
		return 0, fmt.Errorf("invalid type for interface: %T", v)
	}
}

func getPrototype(code byte) (interface{}, error) {
	switch code {
	case CodeExecutionData:
		return &ExecutionData{}, nil
	case CodeRecursiveCIDs:
		return &[]cid.Cid{}, nil
	default:
		return nil, fmt.Errorf("invalid code: %v", code)
	}
}

// serializer encapulates the serialization and deserialization of data
type serializer struct {
	codec      encoding.Codec
	compressor network.Compressor
}

func (s *serializer) writePrototype(w io.Writer, v interface{}) error {
	var code byte
	var err error

	if code, err = getCode(v); err != nil {
		return err
	}

	if bw, ok := w.(io.ByteWriter); ok {
		err = bw.WriteByte(code)
	} else {
		_, err = w.Write([]byte{code})
	}

	if err != nil {
		return fmt.Errorf("failed to write code: %w", err)
	}

	return nil
}

func (s *serializer) Serialize(w io.Writer, v interface{}) error {
	if err := s.writePrototype(w, v); err != nil {
		return fmt.Errorf("failed to write prototype: %w", err)
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

func (s *serializer) readPrototype(r io.Reader) (interface{}, error) {
	var code byte
	var err error

	if br, ok := r.(io.ByteReader); ok {
		code, err = br.ReadByte()
	} else {
		var buf [1]byte
		_, err = r.Read(buf[:])
		code = buf[0]
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read code: %w", err)
	}

	return getPrototype(code)
}

func (s *serializer) Deserialize(r io.Reader) (interface{}, error) {
	v, err := s.readPrototype(r)

	if err != nil {
		return nil, fmt.Errorf("failed to read prototype: %w", err)
	}

	comp, err := s.compressor.NewReader(r)

	if err != nil {
		return nil, fmt.Errorf("failed to create compressor reader: %w", err)
	}

	dec := s.codec.NewDecoder(comp)

	if err := dec.Decode(v); err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return v, nil
}
