package badger

import (
	"fmt"
	"github.com/rs/zerolog/log"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Payloads struct {
	db         *badger.DB
	index      *Index
	guarantees *Guarantees
	seals      *Seals
	receipts   *ExecutionReceipts
	results    *ExecutionResults
}

func NewPayloads(db *badger.DB, index *Index, guarantees *Guarantees, seals *Seals, receipts *ExecutionReceipts) *Payloads {

	p := &Payloads{
		db:         db,
		index:      index,
		guarantees: guarantees,
		seals:      seals,
		receipts:   receipts,
	}

	return p
}

func (p *Payloads) storeTx(blockID flow.Identifier, payload *flow.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// make sure all payload guarantees are stored
		for _, guarantee := range payload.Guarantees {
			err := p.guarantees.storeTx(guarantee)(tx)
			if err != nil {
				return fmt.Errorf("could not store guarantee: %w", err)
			}
		}

		// make sure all payload seals are stored
		for _, seal := range payload.Seals {
			err := p.seals.storeTx(seal)(tx)
			if err != nil {
				return fmt.Errorf("could not store seal: %w", err)
			}
		}

		resultsById := payload.ResultsById()

		// At this point we can be sure that execution result is part of the payload or it's already
		// stored in storage. If execution result is not present in both of those places it means that there is
		// a protocol violation and we are in inconsistent state.
		receiptFromMeta := func(meta *flow.ExecutionReceiptMeta) *flow.ExecutionReceipt {
			if result, ok := resultsById[meta.ResultID]; ok {
				return flow.ExecutionReceiptFromMeta(*meta, *result)
			}

			result, err := p.results.ByID(meta.ResultID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not retrieve result %v from storage", meta.ResultID)
			}
			return flow.ExecutionReceiptFromMeta(*meta, *result)
		}

		// store all payload receipts
		for _, meta := range payload.Receipts {
			// ATTENTION: this is broken from perspective if we have execution receipt which points an execution result
			// which is not included in current payload but was incorporated in one of previous blocks.
			// TODO: refactor receipt/results storages to support new type of storing/retrieving where execution receipt
			// and execution result is decoupled.
			receipt := receiptFromMeta(meta)
			err := p.receipts.store(receipt)(tx)
			if err != nil {
				return fmt.Errorf("could not store receipt: %w", err)
			}
		}

		// store the index
		err := p.index.storeTx(blockID, payload.Index())(tx)
		if err != nil {
			return fmt.Errorf("could not store index: %w", err)
		}

		return nil
	}
}

func (p *Payloads) retrieveTx(blockID flow.Identifier) func(tx *badger.Txn) (*flow.Payload, error) {
	return func(tx *badger.Txn) (*flow.Payload, error) {

		// retrieve the index
		idx, err := p.index.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve index: %w", err)
		}

		// retrieve guarantees
		guarantees := make([]*flow.CollectionGuarantee, 0, len(idx.CollectionIDs))
		for _, collID := range idx.CollectionIDs {
			guarantee, err := p.guarantees.retrieveTx(collID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
			}
			guarantees = append(guarantees, guarantee)
		}

		// retrieve seals
		seals := make([]*flow.Seal, 0, len(idx.SealIDs))
		for _, sealID := range idx.SealIDs {
			seal, err := p.seals.retrieveTx(sealID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
			}
			seals = append(seals, seal)
		}

		// retrieve receipts
		receipts := make([]*flow.ExecutionReceipt, 0, len(idx.ReceiptIDs))
		// multiple receipts can refer to one execution result, avoid duplicating by using map
		resultsLookup := make(map[flow.Identifier]*flow.ExecutionResult)
		for _, recID := range idx.ReceiptIDs {
			receipt, err := p.receipts.byID(recID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve receipt (%x): %w", recID, err)
			}
			receipts = append(receipts, receipt)
			if _, ok := resultsLookup[receipt.ExecutionResult.ID()]; !ok {
				resultsLookup[receipt.ExecutionResult.ID()] = &receipt.ExecutionResult
			}
		}

		metas := make([]*flow.ExecutionReceiptMeta, len(receipts))
		results := make([]*flow.ExecutionResult, 0, len(resultsLookup))
		for i, receipt := range receipts {
			meta := receipt.Meta()
			metas[i] = meta
			// we need to do this to preserve order of results in payload
			if result, found := resultsLookup[meta.ResultID]; found {
				results = append(results, result)
				delete(resultsLookup, meta.ResultID)
			}
		}

		payload := &flow.Payload{
			Seals:      seals,
			Guarantees: guarantees,
			Receipts:   metas,
			Results:    results,
		}

		return payload, nil
	}
}

func (p *Payloads) Store(blockID flow.Identifier, payload *flow.Payload) error {
	return operation.RetryOnConflict(p.db.Update, p.storeTx(blockID, payload))
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	tx := p.db.NewTransaction(false)
	defer tx.Discard()
	return p.retrieveTx(blockID)(tx)
}
