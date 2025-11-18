package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Payloads struct {
	db         storage.DB
	index      *Index
	guarantees *Guarantees
	seals      *Seals
	receipts   *ExecutionReceipts
	results    *ExecutionResults
}

func NewPayloads(db storage.DB, index *Index, guarantees *Guarantees, seals *Seals, receipts *ExecutionReceipts,
	results *ExecutionResults) *Payloads {

	p := &Payloads{
		db:         db,
		index:      index,
		guarantees: guarantees,
		seals:      seals,
		receipts:   receipts,
		results:    results,
	}

	return p
}

// storeTx stores the payloads and their components in the database.
func (p *Payloads) storeTx(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *flow.Payload) error {
	// For correct payloads, the execution result is part of the payload or it's already stored
	// in storage. If execution result is not present in either of those places, we error:
	resultsByID := payload.Results.Lookup()
	fullReceipts := make([]*flow.ExecutionReceipt, 0, len(payload.Receipts))
	var err error
	for _, meta := range payload.Receipts {
		result, ok := resultsByID[meta.ResultID]
		if !ok {
			result, err = p.results.ByID(meta.ResultID)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return fmt.Errorf("invalid payload referencing unknown execution result %v, err: %w", meta.ResultID, err)
				}
				return err
			}
		}
		executionReceipt, err := flow.ExecutionReceiptFromStub(*meta, *result)
		if err != nil {
			return fmt.Errorf("could not create execution receipt from stub: %w", err)
		}

		fullReceipts = append(fullReceipts, executionReceipt)
	}

	// make sure all payload guarantees are stored
	err = p.guarantees.storeGuarantees(payload.Guarantees)(lctx, rw)
	if err != nil {
		return fmt.Errorf("could not store guarantees: %w", err)
	}

	// make sure all payload seals are stored
	for _, seal := range payload.Seals {
		err := p.seals.storeTx(rw, seal)
		if err != nil {
			return fmt.Errorf("could not store seal: %w", err)
		}
	}

	// store all payload receipts
	for _, receipt := range fullReceipts {
		err := p.receipts.storeTx(rw, receipt)
		if err != nil {
			return fmt.Errorf("could not store receipt: %w", err)
		}
	}

	// store the index
	err = p.index.storeTx(lctx, rw, blockID, payload.Index())
	if err != nil {
		return fmt.Errorf("could not store index: %w", err)
	}

	return nil
}

func (p *Payloads) retrieveTx(blockID flow.Identifier) (*flow.Payload, error) {
	// retrieve the index
	idx, err := p.index.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve index: %w", err)
	}

	// retrieve guarantees
	guarantees := make([]*flow.CollectionGuarantee, 0, len(idx.GuaranteeIDs))
	for _, collID := range idx.GuaranteeIDs {
		guarantee, err := p.guarantees.retrieveTx(collID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
		}
		guarantees = append(guarantees, guarantee)
	}

	// retrieve seals
	seals := make([]*flow.Seal, 0, len(idx.SealIDs))
	for _, sealID := range idx.SealIDs {
		seal, err := p.seals.retrieveTx(sealID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
		}
		seals = append(seals, seal)
	}

	// retrieve receipts
	receipts := make([]*flow.ExecutionReceiptStub, 0, len(idx.ReceiptIDs))
	for _, recID := range idx.ReceiptIDs {
		receipt, err := p.receipts.byID(recID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve receipt %x: %w", recID, err)
		}
		receipts = append(receipts, receipt.Stub())
	}

	// retrieve results
	results := make([]*flow.ExecutionResult, 0, len(idx.ResultIDs))
	for _, resID := range idx.ResultIDs {
		result, err := p.results.byID(resID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve result %x: %w", resID, err)
		}
		results = append(results, result)
	}
	payload, err := flow.NewPayload(
		flow.UntrustedPayload{
			Seals:           seals,
			Guarantees:      guarantees,
			Receipts:        receipts,
			Results:         results,
			ProtocolStateID: idx.ProtocolStateID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build the payload: %w", err)
	}

	return payload, nil
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	return p.retrieveTx(blockID)
}
