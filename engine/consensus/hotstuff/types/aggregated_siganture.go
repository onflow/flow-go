package types

type AggregatedSignature struct {
	// RawSignature is the raw bytes of the signature. The signature might be a aggregated
	// BLS signature or a combination of aggregated BLS signature and threshold signature.
	// In order to abstract the detail, this field is defined as []byte. The SigAggregator
	// will be responsible for the encoding/decoding between the actual signature type and
	// raw bytes.
	RawSignature []byte
	// SignerIndexes is the indexes of all the signers. The signers public key can be determined
	// by signerIndex, the cluster filter and the blockID.
	SignerIndexes []byte
}
