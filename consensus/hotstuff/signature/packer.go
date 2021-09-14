package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusSigDataPacker struct {
	committees hotstuff.Committee
}

var _ hotstuff.Packer = &ConsensusSigDataPacker{}

func NewConsensusSigDataPacker(committees hotstuff.Committee) *ConsensusSigDataPacker {
	return &ConsensusSigDataPacker{
		committees: committees,
	}
}

func (p *ConsensusSigDataPacker) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	panic("to be implemented")
}

func (p *ConsensusSigDataPacker) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	panic("to be implemented")
}
