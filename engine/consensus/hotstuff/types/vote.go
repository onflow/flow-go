package types

type Vote struct {
	View      uint64
	BlockMRH  []byte
	Signature *Signature
}
