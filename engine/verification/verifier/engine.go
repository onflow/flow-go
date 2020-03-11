package verifier

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	unit        *engine.Unit                  // used to control startup/shutdown
	log         zerolog.Logger                // used to log relevant actions
	conduit     network.Conduit               // used to propagate result approvals
	me          module.Local                  // used to access local node information
	state       protocol.State                // used to access the protocol state
	rah         crypto.Hasher                 // used as hasher to sign the result approvals
	vm          virtualmachine.VirtualMachine // used to execute transactions
	ledgerDepth int
}

// New creates and returns a new instance of a verifier engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
) (*Engine, error) {

	rt := runtime.NewInterpreterRuntime()

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log,
		state:       state,
		me:          me,
		vm:          virtualmachine.New(rt),
		rah:         utils.NewResultApprovalHasher(),
		ledgerDepth: 257, // TODO (Ramtin) move this to the network config
	}

	var err error
	e.conduit, err = net.Register(engine.ApprovalProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on approval provider channel: %w", err)
	}

	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process receives and submits an event to the verifier engine for processing.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *verification.VerifiableChunk:
		return e.verify(originID, resource)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// verify handles the core verification process. It accepts a verifiable chunk
// and all dependent resources, verifies the chunk, and emits a
// result approval if applicable.
//
// If any part of verification fails, an error is returned, indicating to the
// initiating engine that the verification must be re-tried.
func (e *Engine) verify(originID flow.Identifier, chunk *verification.VerifiableChunk) error {

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote origin for verify")
	}
	// extracts list of verifier nodes id
	//
	// TODO state extraction should be done based on block references
	// https://github.com/dapperlabs/flow-go/issues/2787
	consensusNodes, err := e.state.Final().
		Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not load consensus node IDs: %w", err)
	}

	// extracts chunk ID
	chunkID := chunk.Receipt.ExecutionResult.Chunks.ByIndex(chunk.ChunkIndex).ID()
	// executes assigned chunks
	computedEndState, err := e.executeChunk(chunk)
	if err != nil {
		return fmt.Errorf("could not execute chunk (id=%s): %w", chunkID, err)
	}
	// TODO for now, we discard the computed end state and approve the ER
	_ = computedEndState

	// prepares and signs result approval body part
	body := flow.ResultApprovalBody{
		ExecutionResultID: chunk.Receipt.ExecutionResult.ID(),
		ChunkIndex:        chunk.ChunkIndex,
	}

	// generates a signature over the attestation part of approval
	sign, err := e.signAttestation(&body)
	if err != nil {
		return fmt.Errorf("could not generate attestation signature: %w", err)
	}

	approval := &flow.ResultApproval{
		ResultApprovalBody: body,
		VerifierSignature:  sign,
	}

	// broadcast result approval to consensus nodes
	err = e.conduit.Submit(approval, consensusNodes.NodeIDs()...)
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not submit result approval: %w", err)
	}
	e.log.Info().
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("submitted result approval")

	return nil
}

// executeChunk executes the transactions for a single chunk and returns the
// resultant end state, or an error if execution failed.
// TODO unit testing
func (e *Engine) executeChunk(res *verification.VerifiableChunk) (flow.StateCommitment, error) {
	blockCtx := e.vm.NewBlockContext(&res.Block.Header)

	// TODO ramtin (clean up this) - create a PTrie storage in ledger
	ptrie, err := trie.NewPSMT(res.ChunkDataPack.StartState,
		e.ledgerDepth,
		res.ChunkDataPack.Registers(),
		res.ChunkDataPack.Values(),
		*trie.DecodeProof(res.ChunkDataPack.Proofs()),
	)
	if err != nil {
		return nil, fmt.Errorf("error constructing partial trie %x", err)
	}

	regMap := res.ChunkDataPack.GetRegisterValues()
	getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
		val, ok := regMap[string(key)]
		if !ok {
			return nil, fmt.Errorf("missing register")
		}
		return val, nil
	}

	chunkView := state.NewView(getRegister)

	// TODO check the number of transactions and computation used

	// executes all transactions in this chunk
	for _, tx := range res.Collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	// Apply delta to ptrie
	regs, values := chunkView.Delta().RegisterUpdates()
	expectedEndState, err := ptrie.Update(regs, values)
	if err != nil {
		return nil, fmt.Errorf("error updating partial trie %v", err)

	}
	// Check state commitment
	if !bytes.Equal(expectedEndState, res.EndState) {
		return nil, fmt.Errorf("final state commitment doesn't match: [%x] != [%x]", ptrie.GetRootHash(), res.EndState)
	}

	// TODO compute and return state commitment

	return ptrie.GetRootHash(), nil
}

// signAttestation extracts, signs and returns the attestation part of the result approval
func (e *Engine) signAttestation(body *flow.ResultApprovalBody) (crypto.Signature, error) {
	// extracts attestation part
	atst := body.Attestation()

	// encodes attestation into bytes
	batst, err := encoding.DefaultEncoder.Encode(atst)
	if err != nil {
		return nil, fmt.Errorf("could encode attestation: %w", err)
	}

	// signs attestation
	signature, err := e.me.Sign(batst, e.rah)
	if err != nil {
		return nil, fmt.Errorf("could not sign attestation: %w", err)
	}

	return signature, nil
}
