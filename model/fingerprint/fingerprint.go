package fingerprint

import (
	"github.com/onflow/flow-go/model/encoding/rlp"
)

// Fingerprinter is a type that allows customization of the data used for the fingerprint of the entity. If a type does
// not implement Fingerprinter, RLP encoding is used.
type Fingerprinter interface {
	Fingerprint() []byte
}

// Fingerprint returns a unique byte representation of the passed interface, which can be used as a pre-image for
// hashing, signing or creating IDs/identifiers of the entity. By default, MakeID uses RLP to encode the data. If the
// input defines its own canonical encoding by implementing Fingerprinter, it uses that instead. That allows removal of
// non-unique fields from structs or overwriting of the used encoder. Fingerprint servers two purposes: a) JSON (the
// default encoding) does not specify an order for the elements of arrays and objects, which could lead to different
// hashes of the same entity depending on the JSON implementation and b) the Fingerprinter interface allows to exclude
// fields not needed in the pre-image of the hash that comprises the Identifier, which could be different from the
// encoding for sending entities in messages or for storing them.
func Fingerprint(entity interface{}) []byte {
	if fingerprinter, ok := entity.(Fingerprinter); ok {
		return fingerprinter.Fingerprint()
	}

	return rlp.NewMarshaler().MustMarshal(entity)
}
