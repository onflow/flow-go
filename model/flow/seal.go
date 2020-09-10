// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// AggregatedSignature contains all approvers attenstation signature and verifier IDs per chunk
type AggregatedSignature struct {
	VerifierSignatures []crypto.Signature // TODO: this will later be replaced by a sinlge aggregated signature once we have implemented BLS aggregation
	SignerIDs          []Identifier       // The Identifiers of all the signers
}

type Seal struct {
	BlockID                Identifier
	ResultID               Identifier
	InitialState           StateCommitment
	FinalState             StateCommitment
	AggregatedApprovalSigs []AggregatedSignature
}

func (s Seal) Body() interface{} {
	return struct {
		BlockID      Identifier
		ResultID     Identifier
		InitialState StateCommitment
		FinalState   StateCommitment
	}{
		BlockID:      s.BlockID,
		ResultID:     s.ResultID,
		InitialState: s.InitialState,
		FinalState:   s.FinalState,
	}
}

func (s Seal) ID() Identifier {
	return MakeID(s.Body())
}

func (s Seal) Checksum() Identifier {
	return MakeID(s)
}
