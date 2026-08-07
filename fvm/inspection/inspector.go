package inspection

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

// Inspector is run after each procedure on the procedure output and the starting state of a procedure
// It will then fill out the ProcedureOutput.Inspection results
type Inspector interface {
	// Inspect
	// - storage is the execution state before the procedure was executed.
	//    only the executionSnapshot.Reads, will be read
	// - executionSnapshot is the reads and writes of the procedure
	// - events are all of the events the procedure is emitting
	// - signers are the transaction's authorizing signers, as returned by
	//   [AuthorizingSigners]. Empty for procedures that are not transactions
	//   (e.g. scripts).
	Inspect(
		logger zerolog.Logger,
		storage snapshot.StorageSnapshot,
		executionSnapshot *snapshot.ExecutionSnapshot,
		events []flow.Event,
		signers []flow.Address,
	) (Result, error)

	// Name is the name of the inspector
	Name() string
}

// Result is the result of a procedure inspector
type Result interface {
	InspectionName() string
	AsLogEvent() (zerolog.Level, func(e *zerolog.Event))
}

// AuthorizingSigners returns the deduplicated addresses whose signatures
// authorize the transaction's actions: the proposer followed by the
// authorizers (in insertion order). If the same account appears in multiple
// roles, only its first occurrence is included. Empty addresses (e.g. an unset
// proposer on the system transaction) are omitted.
//
// The payer is deliberately excluded: its signature only authorizes paying the
// transaction's fees, not the transaction's actions.
//
// A nil transaction body returns nil, allowing callers holding an optional
// *TransactionBody (e.g. a non-transaction procedure such as a script) to call
// this function without a nil check.
func AuthorizingSigners(tb *flow.TransactionBody) []flow.Address {
	if tb == nil {
		return nil
	}

	signers := make([]flow.Address, 0, len(tb.Authorizers)+1)
	seen := make(map[flow.Address]struct{})

	addSigner := func(address flow.Address) {
		if address == flow.EmptyAddress {
			return
		}
		if _, ok := seen[address]; ok {
			return
		}
		signers = append(signers, address)
		seen[address] = struct{}{}
	}

	addSigner(tb.ProposalKey.Address)
	for _, authorizer := range tb.Authorizers {
		addSigner(authorizer)
	}

	return signers
}
