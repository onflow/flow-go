package operation

import (
	"errors"
	"fmt"

	"github.com/golang/snappy"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
)

var errUncompressedValue = errors.New("could not uncompress data")

var compressEnabled = true

func setCompressDisabled() {
	compressEnabled = false
}

// encodeEntity encodes the given entity using msgpack and then compress the
// value depending on the global flag.
// possible error to return is irrecoverable.exception
func encodeEntity(entity interface{}) ([]byte, error) {
	if compressEnabled {
		return encodeAndCompress(entity)
	}
	return encodeEntityRaw(entity)
}

// decodeValue decodes the given value into the given entity using msgpack.
// possible error to return is irrecoverable.exception
func decodeValue(val []byte, entity interface{}) error {
	if compressEnabled {
		return decodeCompressed(val, entity)
	}
	return decodeValRaw(val, entity)
}

func encodeEntityRaw(entity interface{}) ([]byte, error) {
	val, err := msgpack.Marshal(entity)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not encode entity: %w", err)
	}
	return val, nil
}

func decodeValRaw(val []byte, entity interface{}) error {
	// decode the entity using msgpack
	err := msgpack.Unmarshal(val, entity)
	if err != nil {
		return irrecoverable.NewExceptionf("could not decode entity: %w", err)
	}
	return nil
}

func encodeAndCompress(entity interface{}) ([]byte, error) {
	// serialize the entity data
	val, err := encodeEntityRaw(entity)
	if err != nil {
		return nil, err
	}

	// compress the serialized data using Snappy
	return snappy.Encode(nil, val), nil
}

func decodeCompressed(val []byte, entity interface{}) error {
	// uncompress the value using Snappy
	uncompressedVal, err := snappy.Decode(nil, val)
	if err != nil {
		return fmt.Errorf("%s: %w", err, errUncompressedValue)
	}

	return decodeValRaw(uncompressedVal, entity)
}

func isErrUncompressedValue(err error) bool {
	return errors.Is(err, errUncompressedValue)
}
