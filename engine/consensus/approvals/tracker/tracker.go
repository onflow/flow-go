package tracker

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
)

// SealingTracker is an auxiliary component for tracking progress of the sealing
// logic (specifically sealing.Core). It has access to the storage, to collect data
// that is not be available directly from sealing.Core. The SealingTracker is immutable
// and therefore intrinsically thread safe.
//
// The SealingTracker essentially acts as a factory for individual SealingObservations,
// which capture information about the progress of a _single_ go routine. Consequently,
// SealingObservations don't need to be concurrency safe, as they are supposed to
// be thread-local structure.
type SealingTracker struct {
	log        zerolog.Logger
	headersDB  storage.Headers
	receiptsDB storage.ExecutionReceipts
	sealsPl    mempool.IncorporatedResultSeals
}

func NewSealingTracker(log zerolog.Logger, headersDB storage.Headers, receiptsDB storage.ExecutionReceipts, sealsPl mempool.IncorporatedResultSeals) *SealingTracker {
	return &SealingTracker{
		log:        log.With().Str("engine", "sealing.SealingTracker").Logger(),
		headersDB:  headersDB,
		receiptsDB: receiptsDB,
		sealsPl:    sealsPl,
	}
}

// nextUnsealedFinalizedBlock determines the ID of the finalized but unsealed
// block with smallest height. It returns an Identity filter that only accepts
// the respective ID.
// In case the next unsealed block has not been finalized, we return the
// False-filter (or if we encounter any problems).
func (st *SealingTracker) nextUnsealedFinalizedBlock(sealedBlock *flow.Header) flow.IdentifierFilter {
	nextUnsealedHeight := sealedBlock.Height + 1
	nextUnsealed, err := st.headersDB.ByHeight(nextUnsealedHeight)
	if err != nil {
		return id.False
	}
	return id.Is(nextUnsealed.ID())
}

// NewSealingObservation constructs a SealingObservation, which capture information
// about the progress of a _single_ go routine. Consequently, SealingObservations
// don't need to be concurrency safe, as they are supposed to be thread-local structure.
func (st *SealingTracker) NewSealingObservation(finalizedBlock *flow.Header, seal *flow.Seal, sealedBlock *flow.Header) consensus.SealingObservation {
	return &SealingObservation{
		SealingTracker:      st,
		startTime:           time.Now(),
		finalizedBlock:      finalizedBlock,
		latestFinalizedSeal: seal,
		latestSealedBlock:   sealedBlock,
		isRelevant:          st.nextUnsealedFinalizedBlock(sealedBlock),
		records:             make(map[flow.Identifier]*SealingRecord),
	}
}

// SealingObservation captures information about the progress of a _single_ go routine.
// Consequently, it is _not concurrency safe_, as SealingObservation is intended to be
// a thread-local structure.
// SealingObservation is supposed to track the status of various (unsealed) incorporated
// results, which sealing.Core processes (driven by that single goroutine).
type SealingObservation struct {
	*SealingTracker

	finalizedBlock      *flow.Header
	latestFinalizedSeal *flow.Seal
	latestSealedBlock   *flow.Header

	startTime  time.Time                          // time when this instance was created
	isRelevant flow.IdentifierFilter              // policy to determine for which blocks we want to track
	records    map[flow.Identifier]*SealingRecord // each record is for one (unsealed) incorporated result
}

// QualifiesForEmergencySealing captures whether sealing.Core has
// determined that the incorporated result qualifies for emergency sealing.
func (st *SealingObservation) QualifiesForEmergencySealing(ir *flow.IncorporatedResult, emergencySealable bool) {
	if !st.isRelevant(ir.Result.BlockID) {
		return
	}
	st.getOrCreateRecord(ir).QualifiesForEmergencySealing(emergencySealable)
}

// ApprovalsMissing captures whether sealing.Core has determined that
// some approvals are still missing for the incorporated result. Calling this
// method with empty `chunksWithMissingApprovals` indicates that all chunks
// have sufficient approvals.
func (st *SealingObservation) ApprovalsMissing(ir *flow.IncorporatedResult, chunksWithMissingApprovals map[uint64]flow.IdentifierList) {
	if !st.isRelevant(ir.Result.BlockID) {
		return
	}
	st.getOrCreateRecord(ir).ApprovalsMissing(chunksWithMissingApprovals)
}

// ApprovalsRequested captures the number of approvals that the business
// logic has re-requested for the incorporated result.
func (st *SealingObservation) ApprovalsRequested(ir *flow.IncorporatedResult, requestCount uint) {
	if !st.isRelevant(ir.Result.BlockID) {
		return
	}
	st.getOrCreateRecord(ir).ApprovalsRequested(requestCount)
}

// getOrCreateRecord returns the sealing record for the given incorporated result.
// If no such record is found, a new record is created and stored in `records`.
func (st *SealingObservation) getOrCreateRecord(ir *flow.IncorporatedResult) *SealingRecord {
	irID := ir.ID()
	record, found := st.records[irID]
	if !found {
		record = &SealingRecord{
			SealingObservation: st,
			IncorporatedResult: ir,
			entries:            make(Rec),
		}
		st.records[irID] = record
	}
	return record
}

// Complete is supposed to be called when a single execution of the sealing logic
// has been completed. It compiles the information about the incorporated results.
func (st *SealingObservation) Complete() {
	observation := st.log.Info()

	// basic information
	observation.Str("finalized_block", st.finalizedBlock.ID().String()).
		Uint64("finalized_block_height", st.finalizedBlock.Height).
		Uint("seals_mempool_size", st.sealsPl.Size())

	// details about the latest finalized seal
	sealDetails, err := st.latestFinalizedSealInfo()
	if err != nil {
		st.log.Error().Err(err).Msg("failed to marshal latestFinalizedSeal details")
	} else {
		observation = observation.Str("finalized_seal", sealDetails)
	}

	// details about the unsealed results that are next
	recList := make([]Rec, 0, len(st.records))
	for irID, rec := range st.records {
		r, err := rec.Generate()
		if err != nil {
			st.log.Error().Err(err).
				Str("incorporated_result", irID.String()).
				Msg("failed to generate sealing record")
			continue
		}
		recList = append(recList, r)
	}
	if len(recList) > 0 {
		bytes, err := json.Marshal(recList)
		if err != nil {
			st.log.Error().Err(err).Msg("failed to marshal records")
		}
		observation = observation.Str("next_unsealed_results", string(bytes))
	}

	// dump observation to Logger
	observation = observation.Int64("duration_ms", time.Since(st.startTime).Milliseconds())
	observation.Msg("sealing observation")
}

// latestFinalizedSealInfo returns a json string representation with the most
// relevant data about the latest finalized seal
func (st *SealingObservation) latestFinalizedSealInfo() (string, error) {
	r := make(map[string]interface{})
	r["executed_block_id"] = st.latestFinalizedSeal.BlockID.String()
	r["executed_block_height"] = st.latestSealedBlock.Height
	r["result_id"] = st.latestFinalizedSeal.ResultID.String()
	r["result_final_state"] = hex.EncodeToString(st.latestFinalizedSeal.FinalState[:])

	bytes, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
