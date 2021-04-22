package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

// TODO add status, events as repeated, gas used, ErrorMessage , register touches
type transactionInContext struct {
	TxIDHex                string   `json:"tx_id_hex"`
	TxIndex                uint64   `json:"tx_index"`
	CollectionIDHex        string   `json:"col_id_hex"`
	CollectionIndex        uint64   `json:"col_index"`
	BlockIDHex             string   `json:"block_id_hex"`
	BlockHeight            uint64   `json:"block_height"`
	ScriptHex              string   `json:"script_hex"`
	ReferenceBlockIDHex    string   `json:"reference_block_id_hex"`
	ArgumentsHex           []string `json:"arguments_hex"`
	GasLimit               uint64   `json:"gas_limit"`
	PayerAddressHex        string   `json:"payer_address_hex"`
	ProposalKeyAddressHex  string   `json:"proposal_key_address_hex"`
	ProposalKeyID          uint64   `json:"proposal_key_id"`
	ProposalSequenceNumber uint64   `json:"proposal_sequence_number"`
	AuthorizersAddressHex  []string `json:"authorizers_address_hex"`
	EnvelopeSize           int      `json:"envelope_size"`
	// Address(hex) + ~ + SignerIndex + ~ + KeyIndex + ~ + Signature(hex)
	CompactPayloadSignatures  []string `json:"compact_payload_signatures"`
	CompactEnvelopeSignatures []string `json:"compact_envelope_signatures"`
	// ErrorMessage              string   `json:"error_message"`
}

// ExportExecutedTransactions exports executed transactions
func ExportExecutedTransactions(blockID flow.Identifier, dbPath string, outputPath string) error {

	// traverse backward from the given block (parent block) and fetch by blockHash
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	index := badger.NewIndex(cacheMetrics, db)
	guarantees := badger.NewGuarantees(cacheMetrics, db, badger.DefaultCacheSize)
	seals := badger.NewSeals(cacheMetrics, db)
	results := badger.NewExecutionResults(cacheMetrics, db)
	receipts := badger.NewExecutionReceipts(cacheMetrics, db, results, badger.DefaultCacheSize)
	transactions := badger.NewTransactions(cacheMetrics, db)
	headers := badger.NewHeaders(cacheMetrics, db)
	payloads := badger.NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := badger.NewBlocks(db, headers, payloads)
	collections := badger.NewCollections(db, transactions)

	activeBlockID := blockID
	outputFile := filepath.Join(outputPath, "transactions.jsonl")

	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create transaction output file %w", err)
	}
	defer fi.Close()

	txWriter := bufio.NewWriter(fi)
	defer txWriter.Flush()

	for {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			return nil
		}

		block, err := blocks.ByID(activeBlockID)
		if err != nil {
			return fmt.Errorf("could not load block %w", err)
		}

		for i, g := range block.Payload.Guarantees {
			col, err := collections.ByID(g.CollectionID)
			if err != nil {
				return fmt.Errorf("could not fetch collection %w", err)
			}
			for j, tx := range col.Transactions {
				txID := tx.ID()
				colID := col.ID()

				args := make([]string, 0)
				for _, a := range tx.Arguments {
					arg := hex.EncodeToString(a)
					args = append(args, arg)
				}

				auths := make([]string, 0)
				for _, a := range tx.Authorizers {
					auth := hex.EncodeToString(a[:])
					auths = append(auths, auth)
				}

				psig := make([]string, 0)
				for _, s := range tx.PayloadSignatures {
					cs := hex.EncodeToString(s.Address[:]) + "~" + fmt.Sprint(s.SignerIndex) + "~" + fmt.Sprint(s.KeyIndex) + "~" + hex.EncodeToString(s.Signature)
					psig = append(psig, cs)
				}

				esig := make([]string, 0)
				for _, s := range tx.EnvelopeSignatures {
					cs := hex.EncodeToString(s.Address[:]) + "~" + fmt.Sprint(s.SignerIndex) + "~" + fmt.Sprint(s.KeyIndex) + "~" + hex.EncodeToString(s.Signature)
					esig = append(esig, cs)
				}

				tic := transactionInContext{
					TxIDHex:                   hex.EncodeToString(txID[:]),
					TxIndex:                   uint64(j),
					CollectionIDHex:           hex.EncodeToString(colID[:]),
					CollectionIndex:           uint64(i),
					BlockIDHex:                hex.EncodeToString(activeBlockID[:]),
					BlockHeight:               header.Height,
					ScriptHex:                 hex.EncodeToString(tx.Script),
					ReferenceBlockIDHex:       hex.EncodeToString(tx.ReferenceBlockID[:]),
					ArgumentsHex:              args,
					GasLimit:                  tx.GasLimit,
					PayerAddressHex:           hex.EncodeToString(tx.Payer[:]),
					ProposalKeyAddressHex:     hex.EncodeToString(tx.ProposalKey.Address[:]),
					ProposalKeyID:             tx.ProposalKey.KeyIndex,
					ProposalSequenceNumber:    tx.ProposalKey.SequenceNumber,
					AuthorizersAddressHex:     auths,
					EnvelopeSize:              computeEnvelopeSize(tx),
					CompactPayloadSignatures:  psig,
					CompactEnvelopeSignatures: esig,
				}

				jsonData, err := json.Marshal(tic)
				if err != nil {
					return fmt.Errorf("could not create a json obj for a transaction: %w", err)
				}
				_, err = txWriter.WriteString(string(jsonData) + "\n")
				if err != nil {
					return fmt.Errorf("could not write transaction json to file: %w", err)
				}
				txWriter.Flush()
			}
		}
		activeBlockID = header.ParentID
	}
}

func computeEnvelopeSize(tx *flow.TransactionBody) int {
	// compute envelope size
	envelopeSize := 0
	envelopeSize += 32             // ReferenceBlockID
	envelopeSize += len(tx.Script) // Script
	for _, arg := range tx.Arguments {
		envelopeSize += len(arg) // arg size
	}
	envelopeSize += 8                                        // GasLimit
	envelopeSize += flow.AddressLength + 8 + 8               // ProposalKey
	envelopeSize += flow.AddressLength                       // Payer
	envelopeSize += flow.AddressLength * len(tx.Authorizers) // per authorizer
	for _, sig := range tx.PayloadSignatures {
		envelopeSize += flow.AddressLength + 8 + 8 + len(sig.Signature) // per payload sig
	}
	for _, sig := range tx.EnvelopeSignatures {
		envelopeSize += flow.AddressLength + 8 + 8 + len(sig.Signature) // per env sig
	}
	return envelopeSize
}
