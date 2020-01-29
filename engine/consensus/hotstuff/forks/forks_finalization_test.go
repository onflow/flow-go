package forks

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

func TestEventHandler(t *testing.T) {
	notifier := &mockdist.Distributor{}
	rootBlock *types.BlockProposal, rootQc *types.QuorumCertificate
	finalizer.New(, , notifier)

	forks := New()
	fmt.Printf("%v", eventHandler)
}



func qc(view uint64) *types.QuorumCertificate {
	return &types.QuorumCertificate{View: view}
}

func makeBlockProposal(qcView, blockView uint64) *types.BlockProposal {
	return &types.BlockProposal{
		Block: &types.Block{View: blockView, QC: qc(qcView)},
	}
}
