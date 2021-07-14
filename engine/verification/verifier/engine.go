package verifier

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification/utils"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine (verifier engine) verifies chunks, generates result approvals or raises challenges.
// as input it accepts verifiable chunks (chunk + all data needed) and perform verification by
// constructing a partial trie, executing transactions and check the final state commitment and
// other chunk meta data (e.g. tx count)
type Engine struct {
	unit        *engine.Unit               // used to control startup/shutdown
	log         zerolog.Logger             // used to log relevant actions
	metrics     module.VerificationMetrics // used to capture the performance metrics
	tracer      module.Tracer              // used for tracing
	pushConduit network.Conduit            // used to push result approvals
	pullConduit network.Conduit            // used to respond to requests for result approvals
	me          module.Local               // used to access local node information
	state       protocol.State             // used to access the protocol state
	rah         hash.Hasher                // used as hasher to sign the result approvals
	chVerif     module.ChunkVerifier       // used to verify chunks
	spockHasher hash.Hasher                // used for generating spocks
	approvals   storage.ResultApprovals    // used to store result approvals
}

// New creates and returns a new instance of a verifier engine.
func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	net module.Network,
	state protocol.State,
	me module.Local,
	chVerif module.ChunkVerifier,
	approvals storage.ResultApprovals,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "verifier").Logger(),
		metrics:     metrics,
		tracer:      tracer,
		state:       state,
		me:          me,
		chVerif:     chVerif,
		rah:         utils.NewResultApprovalHasher(),
		spockHasher: crypto.NewBLSKMAC(encoding.SPOCKTag),
		approvals:   approvals,
	}

	var err error
	e.pushConduit, err = net.Register(engine.PushApprovals, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on approval push channel: %w", err)
	}

	e.pullConduit, err = net.Register(engine.ProvideApprovalsByChunk, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on approval pull channel: %w", err)
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
	e.unit.Launch(func() {
		err := e.ProcessLocal(event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process receives verifiable chunks, evaluate them and send them for chunk verifier
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	var err error

	switch resource := event.(type) {
	case *verification.VerifiableChunkData:
		err = e.verifiableChunkHandler(originID, resource)
	case *messages.ApprovalRequest:
		err = e.approvalRequestHandler(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	if err != nil {
		// logs the error instead of returning that.
		// returning error would be projected at a higher level by network layer.
		// however, this is an engine-level error, and not network layer error.
		e.log.Warn().Err(err).Msg("engine could not process event successfully")
	}

	return nil
}

// verify handles the core verification process. It accepts a verifiable chunk
// and all dependent resources, verifies the chunk, and emits a
// result approval if applicable.
//
// If any part of verification fails, an error is returned, indicating to the
// initiating engine that the verification must be re-tried.
func (e *Engine) verify(ctx context.Context, originID flow.Identifier,
	vc *verification.VerifiableChunkData) error {
	// log it first
	log := e.log.With().Timestamp().
		Hex("origin", logging.ID(originID)).
		Uint64("chunk_index", vc.Chunk.Index).
		Hex("result_id", logging.Entity(vc.Result)).
		Logger()

	log.Info().Msg("verifiable chunk received by verifier engine")

	// only accept internal calls
	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote origin for verify")
	}

	var err error

	// extracts chunk ID
	ch, ok := vc.Result.Chunks.ByIndex(vc.Chunk.Index)
	if !ok {
		return engine.NewInvalidInputErrorf("chunk out of range requested: %v", vc.Chunk.Index)
	}
	log.With().Hex("chunk_id", logging.Entity(ch)).Logger()

	// execute the assigned chunk
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERVerChunkVerify)

	var spockSecret []byte
	var chFault chmodels.ChunkFault
	if vc.IsSystemChunk {
		spockSecret, chFault, err = e.chVerif.SystemChunkVerify(vc)
	} else {
		spockSecret, chFault, err = e.chVerif.Verify(vc)
	}
	span.Finish()
	// Any err means that something went wrong when verify the chunk
	// the outcome of the verification is captured inside the chFault and not the err
	if err != nil {
		return fmt.Errorf("cannot verify chunk: %w", err)
	}

	// if any fault found with the chunk
	if chFault != nil {
		switch chFault.(type) {
		case *chmodels.CFMissingRegisterTouch:
			e.log.Warn().Msg(chFault.String())
			// still create approvals for this case
		case *chmodels.CFNonMatchingFinalState:
			// TODO raise challenge
			e.log.Warn().Msg(chFault.String())
			return nil
		case *chmodels.CFInvalidVerifiableChunk:
			// TODO raise challenge
			e.log.Error().Msg(chFault.String())
			return nil
		case *chmodels.CFInvalidEventsCollection:
			// TODO raise challenge
			e.log.Error().Msg(chFault.String())
			return nil
		default:
			return engine.NewInvalidInputErrorf("unknown type of chunk fault is received (type: %T) : %v",
				chFault, chFault.String())
		}
	}

	// Generate result approval
	span, _ = e.tracer.StartSpanFromContext(ctx, trace.VERVerGenerateResultApproval)
	approval, err := e.GenerateResultApproval(vc.Chunk.Index, vc.Result.ID(), vc.Header.ID(), spockSecret)
	span.Finish()
	if err != nil {
		return fmt.Errorf("couldn't generate a result approval: %w", err)
	}

	err = e.approvals.Store(approval)
	if err != nil {
		return fmt.Errorf("could not store approval: %w", err)
	}

	err = e.approvals.Index(approval.Body.ExecutionResultID, approval.Body.ChunkIndex, approval.ID())
	if err != nil {
		return fmt.Errorf("could not index approval: %w", err)
	}

	// Extracting consensus node ids
	// TODO state extraction should be done based on block references
	consensusNodes, err := e.state.Final().
		Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not load consensus node IDs: %w", err)
	}

	// broadcast result approval to the consensus nodes
	err = e.pushConduit.Publish(approval, consensusNodes.NodeIDs()...)
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not submit result approval: %w", err)
	}
	log.Info().Msg("result approval submitted")
	// increases number of sent result approvals for sake of metrics
	e.metrics.OnResultApprovalDispatchedInNetworkByVerifier()

	return nil
}

// GenerateResultApproval generates result approval for specific chunk of an execution receipt.
func (e *Engine) GenerateResultApproval(chunkIndex uint64,
	execResultID flow.Identifier,
	blockID flow.Identifier,
	spockSecret []byte,
) (*flow.ResultApproval, error) {

	// attestation
	atst := flow.Attestation{
		BlockID:           blockID,
		ExecutionResultID: execResultID,
		ChunkIndex:        chunkIndex,
	}

	// generates a signature over the attestation part of approval
	atstID := atst.ID()
	atstSign, err := e.me.Sign(atstID[:], e.rah)
	if err != nil {
		return nil, fmt.Errorf("could not sign attestation: %w", err)
	}

	// generates spock
	spock, err := e.me.SignFunc(spockSecret, e.spockHasher, crypto.SPOCKProve)
	if err != nil {
		return nil, fmt.Errorf("could not generate SPoCK: %w", err)
	}

	// result approval body
	body := flow.ResultApprovalBody{
		Attestation:          atst,
		ApproverID:           e.me.NodeID(),
		AttestationSignature: atstSign,
		Spock:                spock,
	}

	// generates a signature over result approval body
	bodyID := body.ID()
	bodySign, err := e.me.Sign(bodyID[:], e.rah)
	if err != nil {
		return nil, fmt.Errorf("could not sign result approval body: %w", err)
	}

	return &flow.ResultApproval{
		Body:              body,
		VerifierSignature: bodySign,
	}, nil
}

// verifiableChunkHandler acts as a wrapper around the verify method that captures its performance-related metrics
func (e *Engine) verifiableChunkHandler(originID flow.Identifier, ch *verification.VerifiableChunkData) error {
	ctx := context.Background()
	if span, ok := e.tracer.GetSpan(ch.Result.ID(), trace.VERProcessExecutionResult); ok {
		defer span.Finish()
		childSpan := e.tracer.StartSpanFromParent(span, trace.VERVerVerifyWithMetrics)
		ctx = opentracing.ContextWithSpan(ctx, childSpan)
		defer childSpan.Finish()
	}

	// increments number of received verifiable chunks
	// for sake of metrics
	e.metrics.OnVerifiableChunkReceivedAtVerifierEngine()

	log := e.log.With().
		Hex("result_id", logging.ID(ch.Result.ID())).
		Hex("chunk_id", logging.ID(ch.Chunk.ID())).
		Uint64("chunk_index", ch.Chunk.Index).Logger()

	log.Info().Msg("verifiable chunk received")

	// starts verification of chunk
	err := e.verify(ctx, originID, ch)

	if err != nil {
		log.Info().Err(err).Msg("could not verify chunk")
	}

	// closes verification performance metrics trackers
	return nil
}

func (e *Engine) approvalRequestHandler(originID flow.Identifier, req *messages.ApprovalRequest) error {

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("result_id", logging.ID(req.ResultID)).
		Uint64("chunk_index", req.ChunkIndex).
		Logger()

	origin, err := e.state.Final().Identity(originID)
	if err != nil {
		return engine.NewInvalidInputErrorf("invalid origin id (%s): %w", originID, err)
	}

	if origin.Role != flow.RoleConsensus {
		return engine.NewInvalidInputErrorf("invalid role for requesting approvals: %s", origin.Role)
	}

	approval, err := e.approvals.ByChunk(req.ResultID, req.ChunkIndex)
	if err != nil {
		return fmt.Errorf("could not retrieve approval for chunk (result: %s, chunk index: %d): %w",
			req.ResultID,
			req.ChunkIndex,
			err)
	}

	response := &messages.ApprovalResponse{
		Nonce:    req.Nonce,
		Approval: *approval,
	}

	err = e.pullConduit.Unicast(response, originID)
	if err != nil {
		return fmt.Errorf("could not send requested approval to %s: %w",
			originID,
			err)
	}

	log.Debug().Msg("succesfully replied to approval request")

	return nil
}
