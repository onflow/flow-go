package ledger

import "encoding/binary"

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	Key   Key
	Value Value
}

// TODO Ramtin fix the issue with Encode for compute compact value

// TODO add test for this
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
	binary.BigEndian.PutUint64(encVSize, uint64(len(encVSize)))

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

// NewPayload returns a new payload
func NewPayload(key Key, value Value) *Payload {
	return &Payload{Key: key, Value: value}
}

// TODO fix me
func (p *Payload) String() string {
	// TODO Add key, values
	return ""
}

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}
