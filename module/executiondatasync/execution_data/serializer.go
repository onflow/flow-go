package execution_data

import (
	"fmt"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/network"
)

// header codes to distinguish between different types of data
const (
	codeRecursiveCIDs = iota + 1
	codeExecutionDataRoot
	codeChunkExecutionData
)

func getCode(v interface{}) (byte, error) {
	switch v.(type) {
	case *BlockExecutionDataRoot:
		return codeExecutionDataRoot, nil
	case *ChunkExecutionData:
		return codeChunkExecutionData, nil
	case []cid.Cid:
		return codeRecursiveCIDs, nil
	default:
		return 0, fmt.Errorf("invalid type for interface: %T", v)
	}
}

func getPrototype(code byte) (interface{}, error) {
	switch code {
	case codeExecutionDataRoot:
		return &BlockExecutionDataRoot{}, nil
	case codeChunkExecutionData:
		return &ChunkExecutionData{}, nil
	case codeRecursiveCIDs:
		return &[]cid.Cid{}, nil
	default:
		return nil, fmt.Errorf("invalid code: %v", code)
	}
}

// Serializer is used to serialize / deserialize Execution Data and CID lists for the
// Execution Data Service. An object is serialized by encoding and compressing it using
// the given codec and compressor.
//
// The serialized data is prefixed with a single byte header that identifies the underlying
// data format. This allows adding new data types in a backwards compatible way.
type Serializer struct {
	codec      encoding.Codec
	compressor network.Compressor
}

func NewSerializer(codec encoding.Codec, compressor network.Compressor) *Serializer {
	return &Serializer{
		codec:      codec,
		compressor: compressor,
	}
}

// writePrototype writes the header code for the given value to the given writer
func (s *Serializer) writePrototype(w io.Writer, v interface{}) error {
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

// Serialize encodes and compresses the given value to the given writer
func (s *Serializer) Serialize(w io.Writer, v interface{}) error {
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

	// flush data out to the underlying writer
	if err := comp.Close(); err != nil {
		return fmt.Errorf("failed to close compressor: %w", err)
	}

	return nil
}

// readPrototype reads a header code from the given reader and returns a prototype value
func (s *Serializer) readPrototype(r io.Reader) (interface{}, error) {
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

// Deserialize decompresses and decodes the data from the given reader
func (s *Serializer) Deserialize(r io.Reader) (interface{}, error) {
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
