package crypto

// Encoder should be implemented by structures to be hashed

// Encoder is an interface of a generic structure
type Encoder interface {
	Encode() []byte
}
