package ledger

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	Key   Key
	Value Value
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

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}
