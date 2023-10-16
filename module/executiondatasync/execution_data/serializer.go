package execution_data

import (
	"fmt"
	"io"
	"math"

	cborlib "github.com/fxamacker/cbor/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/internal"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
)

// DefaultSerializer is the default implementation for an Execution Data serializer.
// It is configured to use cbor encoding with LZ4 compression.
var DefaultSerializer Serializer

func init() {
	var codec encoding.Codec

	decMode, err := cborlib.DecOptions{
		MaxArrayElements: math.MaxInt64,
		MaxMapPairs:      math.MaxInt64,
		MaxNestedLevels:  math.MaxInt16,
	}.DecMode()

	if err != nil {
		panic(err)
	}

	codec = cbor.NewCodec(cbor.WithDecMode(decMode))
	DefaultSerializer = NewSerializer(codec, compressor.NewLz4Compressor())
}

// header codes are used to distinguish between the different data types serialized within a blob.
// they provide simple versioning of execution state data blobs and indicate how the data should
// be deserialized back into their original form. Therefore, each input format must have a unique
// code, and the codes must never be reused. This allows libraries to accurately decode the data
// without juggling software versions.
const (
	codeRecursiveCIDs = iota + 1
	codeExecutionDataRoot
	codeChunkExecutionDataV1
	codeChunkExecutionDataV2 // includes transaction results
)

// getCode returns the header code for the given value's type.
// It returns an error if the type is not supported.
func getCode(v interface{}) (byte, error) {
	switch v.(type) {
	case *flow.BlockExecutionDataRoot:
		return codeExecutionDataRoot, nil
	case *internal.ChunkExecutionDataV1: // only used for backwards compatibility testing
		return codeChunkExecutionDataV1, nil
	case *ChunkExecutionData:
		return codeChunkExecutionDataV2, nil
	case []cid.Cid:
		return codeRecursiveCIDs, nil
	default:
		return 0, fmt.Errorf("invalid type for interface: %T", v)
	}
}

// getPrototype returns a new instance of the type that corresponds to the given header code.
// It returns an error if the code is not supported.
func getPrototype(code byte) (interface{}, error) {
	switch code {
	case codeExecutionDataRoot:
		return &flow.BlockExecutionDataRoot{}, nil
	case codeChunkExecutionDataV2, codeChunkExecutionDataV1:
		return &ChunkExecutionData{}, nil // only return the latest version
	case codeRecursiveCIDs:
		return &[]cid.Cid{}, nil
	default:
		return nil, fmt.Errorf("invalid code: %v", code)
	}
}

// Serializer is used to serialize / deserialize Execution Data and CID lists for the
// Execution Data Service.
type Serializer interface {
	// Serialize encodes and compresses the given value to the given writer.
	// No errors are expected during normal operation.
	Serialize(io.Writer, interface{}) error

	// Deserialize decompresses and decodes the data from the given reader.
	// No errors are expected during normal operation.
	Deserialize(io.Reader) (interface{}, error)
}

// serializer implements the Serializer interface. Object are serialized by encoding and
// compressing them using the given codec and compressor.
//
// The serialized data is prefixed with a single byte header that identifies the underlying
// data format. This allows adding new data types in a backwards compatible way.
type serializer struct {
	codec      encoding.Codec
	compressor network.Compressor
}

// NewSerializer returns a new Execution Data serializer using the provided encoder and compressor.
func NewSerializer(codec encoding.Codec, compressor network.Compressor) *serializer {
	return &serializer{
		codec:      codec,
		compressor: compressor,
	}
}

// writePrototype writes the header code for the given value to the given writer
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

// Serialize encodes and compresses the given value to the given writer.
// No errors are expected during normal operation.
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

	// flush data out to the underlying writer
	if err := comp.Close(); err != nil {
		return fmt.Errorf("failed to close compressor: %w", err)
	}

	return nil
}

// readPrototype reads a header code from the given reader and returns a prototype value
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

// Deserialize decompresses and decodes the data from the given reader.
// No errors are expected during normal operation.
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
