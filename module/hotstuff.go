package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// HotStuff defines the interface to the core HotStuff algorithm. It includes
// a method to start the event loop, and utilities to submit block proposals
// and votes received from other replicas.
type HotStuff interface {
	ReadyDoneAware

	// SubmitProposal submits a new block proposal to the HotStuff event loop.
	// This method blocks until the proposal is accepted to the event queue.
	//
	// Block proposals must be submitted in order and only if they extend a
	// block already known to HotStuff core.
	SubmitProposal(proposal *flow.Header, parentView uint64)

	// SubmitVote submits a new vote to the HotStuff event loop.
	// This method blocks until the vote is accepted to the event queue.
	//
	// Votes may be submitted in any order.
	SubmitVote(originID flow.Identifier, blockID flow.Identifier, view uint64, stakingSig crypto.Signature, randomBeaconSig crypto.Signature)
}
