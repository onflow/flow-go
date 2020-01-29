package types

type Vote struct {
	UnsignedVote
	Signature *Signature
}

func NewVote(unsigned *UnsignedVote, sig *Signature) *Vote {
	return &Vote{
		UnsignedVote: *unsigned,
		Signature:    sig,
	}
}

func (uv Vote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockID)
}
