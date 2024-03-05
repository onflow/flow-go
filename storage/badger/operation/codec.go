package operation

import (
	"github.com/golang/snappy"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
)

var compressEnabled = true

func encodeEntity(entity interface{}) ([]byte, error) {
	if compressEnabled {
		return encodeAndCompress(entity)
	}
	return encodeEntityRaw(entity)
}

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
		return irrecoverable.NewExceptionf("could not uncompress data: %w", err)
	}

	return decodeValRaw(uncompressedVal, entity)
}
