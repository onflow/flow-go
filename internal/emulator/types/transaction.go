package types

type RawTransaction struct {
	Script       []byte
	Nonce        uint64
	ComputeLimit uint64
}

type SignedTransaction struct {
	Transaction    *RawTransaction
	PayerSignature crypto.Signature
}
