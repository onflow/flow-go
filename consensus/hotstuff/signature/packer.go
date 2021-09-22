package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type SigDataPacker struct {
	committees hotstuff.Committee
}

var _ hotstuff.Packer = &SigDataPacker{}

func NewSigDataPacker(committees hotstuff.Committee) *SigDataPacker {
	return &SigDataPacker{
		committees: committees,
	}
}

func (p *SigDataPacker) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	panic("to be implemented")
}

func (p *SigDataPacker) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	panic("to be implemented")
}
