package def

import (
	"bytes"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/utils"
)

type BlockProposal struct {
	*Block
	// TODO: We will replace Payload with FlattenedPayload later
	// For now we just leave Payload as generic bytes here
	PrimaryVote *crypto.Signature
}

func (bp *BlockProposal) HasValidSignature() bool {
	signedContent := bytes.Join(
		[][]byte{
			bp.Block.BlockMRH,
			utils.ConvertUintToByte(bp.Block.View),
		},
		[]byte{},
	)

	return crypto.VerifySig(signedContent, bp.PrimaryVote)
}

// TODO: replace with cypto function
func computePayloadMRH(pl []byte) []byte {
	return pl
}

// Every message will be signed, so the receiver will
// be able to know the public key of the sender from the signature and
// therefore be able to look up the IP and port of the sender. So no need
// to include querier address in BlockRequest
type BlockRequest struct {
	BlockMRH []byte

	ContentSignature *crypto.Signature
}
