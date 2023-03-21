package attestation

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// Attestor collect and prepare attestation for executed collections
type Attestor interface {
	Attest(
		*state.ExecutionSnapshot,
		flow.StateCommitment,
	) (
		flow.StateCommitment,
		[]byte,
		*ledger.TrieUpdate,
		error,
	)
}
