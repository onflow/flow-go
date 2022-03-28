package encoding

// SigType is the aggregable signature type.
type SigType uint8

// SigType specifies the role of the signature in the protocol.
// Both types are aggregatable cryptographic signatures.
//  * SigTypeRandomBeacon type is for random beacon signatures.
//  * SigTypeStaking is for Hotstuff signatures.
const (
	SigTypeStaking SigType = 0
	SigTypeRandomBeacon = 1
)

// Valid returns true if the signature is either SigTypeStaking or SigTypeRandomBeacon
// else return false
func (t SigType) Valid() bool {
	return t == SigTypeStaking || t == SigTypeRandomBeacon
}
