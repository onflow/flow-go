package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
}

var _ hotstuff.Packer = &ConsensusSigPackerImpl{}

func NewConsensusSigPackerImpl(committees hotstuff.Committee) *ConsensusSigPackerImpl {
	return &ConsensusSigPackerImpl{
		committees: committees,
	}
}

func (p *ConsensusSigPackerImpl) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	panic("to be implemented")
}

func (p *ConsensusSigPackerImpl) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	panic("to be implemented")
}
