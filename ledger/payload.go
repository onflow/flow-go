package ledger

import (
	"encoding/binary"
	"fmt"
)

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	Key   Key
	Value Value
}

// Encode encodes the the payload object
func (p *Payload) Encode() []byte {
	ret := make([]byte, 0)

	// encode key first
	encK := p.Key.Encode()
	// include the size of encoded key
	encKSize := make([]byte, 2)
	binary.BigEndian.PutUint16(encKSize, uint16(len(encK)))
	ret = append(ret, encKSize...)

	// then include the encoded key
	ret = append(ret, encK...)

	// encode the value next
	encV := p.Value.Encode()

	// include the size of encoded value
	// this has been included for future expansion of struct
	encVSize := make([]byte, 8)
	binary.BigEndian.PutUint64(encVSize, uint64(len(encV)))
	ret = append(ret, encVSize...)

	// include the value
	ret = append(ret, encV...)

	return ret
}

// Size returns the size of the payload
func (p *Payload) Size() int {
	return p.Key.Size() + p.Value.Size()
}

// IsEmpty returns true if key or value is not empty
func (p *Payload) IsEmpty() bool {
	return p.Size() == 0
}

// TODO fix me
func (p *Payload) String() string {
	// TODO Add key, values
	return ""
}

// NewPayload returns a new payload
func NewPayload(key Key, value Value) *Payload {
	return &Payload{Key: key, Value: value}
}

// DecodePayload decodes a payload from an encoded byte slice
func DecodePayload(inp []byte) (*Payload, error) {
	if len(inp) == 0 {
		return nil, nil
	}
	index := 0
	// encoded key size
	encKeySize := binary.BigEndian.Uint16(inp[:2])
	index += 2
	encKey := inp[2 : 2+int(encKeySize)]
	index += int(encKeySize)
	key, err := DecodeKey(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed decoding payload: %w", err)
	}

	encValueSize := binary.BigEndian.Uint64(inp[index : index+8])
	index += 8
	encValue := inp[index : index+int(encValueSize)]
	value, err := DecodeValue(encValue)
	if err != nil {
		return nil, fmt.Errorf("failed decoding payload: %w", err)
	}

	return &Payload{Key: *key, Value: value}, nil
}

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}
