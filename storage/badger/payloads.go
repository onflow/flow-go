package store

import (
	"errors"
	"fmt"
	"maps"

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

// TODO(leo): rename to BatchStore, add to interface
func (p *Payloads) storeTx(rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *flow.Payload, storingResults map[flow.Identifier]*flow.ExecutionResult) error {
	// For correct payloads, the execution result is part of the payload or it's already stored
	// in storage. If execution result is not present in either of those places, we error.
	// ATTENTION: this is unnecessarily complex if we have execution receipt which points an execution result
	// which is not included in current payload but was incorporated in one of previous blocks.

	resultsByID := payload.Results.Lookup()
	fullReceipts := make([]*flow.ExecutionReceipt, 0, len(payload.Receipts))
	var err error
	for _, meta := range payload.Receipts {
		result, ok := resultsByID[meta.ResultID]
		if !ok {
			// check if the result exists in previous blocks that stored within the same batch
			result, ok = storingResults[meta.ResultID]
			if !ok {
				result, err = p.results.ByID(meta.ResultID)
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						err = fmt.Errorf("invalid payload referencing unknown execution result %v, err: %w", meta.ResultID, err)
					}
					return err
				}
			}
		}
		fullReceipts = append(fullReceipts, flow.ExecutionReceiptFromMeta(*meta, *result))
	}

	maps.Copy(storingResults, resultsByID)

	// make sure all payload guarantees are stored
	for _, guarantee := range payload.Guarantees {
		err := p.guarantees.storeTx(rw, guarantee)
		if err != nil {
			return fmt.Errorf("could not store guarantee: %w", err)
		}
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
	err = p.index.storeTx(rw, blockID, payload.Index())
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
	guarantees := make([]*flow.CollectionGuarantee, 0, len(idx.CollectionIDs))
	for _, collID := range idx.CollectionIDs {
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
	receipts := make([]*flow.ExecutionReceiptMeta, 0, len(idx.ReceiptIDs))
	for _, recID := range idx.ReceiptIDs {
		receipt, err := p.receipts.byID(recID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve receipt %x: %w", recID, err)
		}
		receipts = append(receipts, receipt.Meta())
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
	payload := &flow.Payload{
		Seals:           seals,
		Guarantees:      guarantees,
		Receipts:        receipts,
		Results:         results,
		ProtocolStateID: idx.ProtocolStateID,
	}

	return payload, nil
}

func (p *Payloads) Store(blockID flow.Identifier, payload *flow.Payload) error {
	return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return p.storeTx(rw, blockID, payload, make(map[flow.Identifier]*flow.ExecutionResult))
	})
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	return p.retrieveTx(blockID)
}
