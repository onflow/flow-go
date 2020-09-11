package export

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
)

// type Loader struct {
// 	headers          *badger.Headers
// 	index            *badger.Index
// 	guarantees       *badger.Guarantees
// 	events           *badger.Events
// 	seals            *badger.Seals
// 	payloads         *badger.Payloads
// 	commits          *badger.Commits
// 	transactions     *badger.Transactions
// 	collections      *badger.Collections
// 	executionResults *badger.ExecutionResults
// 	blocks           *badger.Blocks
// 	chunkDataPacks   *badger.ChunkDataPacks
// 	executionState   state.ExecutionState
// 	metrics          *metrics.NoopCollector
// 	vm               *fvm.VirtualMachine
// 	ctx              context.Context
// 	mappingMutex     sync.Mutex
// }

// TODO add events

func ExportEvents(blockID flow.Identifier, dbPath string, outputPath string) {
	// TODO
	// blockHash :=
	// traverse backward (parent block) and fetch by blockHash

	fmt.Println(">>>>", dbPath)
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	// tracer := &trace.NoopTracer{}

	events := badger.NewEvents(db)
	index := badger.NewIndex(cacheMetrics, db)
	identities := badger.NewIdentities(cacheMetrics, db)
	guarantees := badger.NewGuarantees(cacheMetrics, db)
	seals := badger.NewSeals(cacheMetrics, db)
	transactions := badger.NewTransactions(cacheMetrics, db)
	headers := badger.NewHeaders(cacheMetrics, db)

	// commits := badger.NewCommits(cacheMetrics, db)
	payloads := badger.NewPayloads(db, index, identities, guarantees, seals)
	blocks := badger.NewBlocks(db, headers, payloads)
	collections := badger.NewCollections(db, transactions)
	// chunkDataPacks := badger.NewChunkDataPacks(db)
	// executionResults := badger.NewExecutionResults(db)
	// executionState := state.NewExecutionState(nil, commits, blocks, collections, chunkDataPacks, executionResults, db, tracer)

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
		// Address(hex) + ~ + SignerIndex + ~ + KeyID + ~ + Signature(hex)
		CompactPayloadSignatures  []string `json:"compact_payload_signatures"`
		CompactEnvelopeSignatures []string `json:"compact_envelope_signatures"`
		ErrorMessage              string   `json:"error_message"`
	}

	// status
	// TODO gas used, status

	// initialRT := initialRuntime.NewInterpreterRuntime()
	// vm := fvm.NewWithInitial(initialRT)

	// loader := Loader{
	// 	headers:          headers,
	// 	index:            index,
	// 	guarantees:       guarantees,
	// 	events:           events,
	// 	seals:            seals,
	// 	payloads:         payloads,
	// 	commits:          commits,
	// 	transactions:     transactions,
	// 	collections:      collections,
	// 	executionResults: executionResults,
	// 	blocks:           blocks,
	// 	chunkDataPacks:   chunkDataPacks,
	// 	executionState:   executionState,
	// 	metrics:          cacheMetrics,
	// 	vm:               vm,
	// 	ctx:              context.Background(),
	// }

	var activeBlockID flow.Identifier
	activeBlockID = blockID
	done := false

	eventOutputFile := outputPath + "/events.txt"
	efi, err := os.Create(eventOutputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create event output file")
	}
	defer efi.Close()
	eventWriter := bufio.NewWriter(efi)
	defer eventWriter.Flush()

	txOutputFile := outputPath + "/transactions.txt"
	tfi, err := os.Create(txOutputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create collection output file")
	}
	defer tfi.Close()
	txWriter := bufio.NewWriter(tfi)
	defer txWriter.Flush()

	for !done {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not load header")
			done = true
		}

		block, err := blocks.ByID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not load block")
			done = true
		}

		for i, g := range block.Payload.Guarantees {
			col, err := collections.ByID(g.CollectionID)
			if err != nil {
				log.Fatal().Err(err).Msg("could not fetch collection")
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
					cs := hex.EncodeToString(s.Address[:]) + "~" + fmt.Sprint(s.SignerIndex) + "~" + fmt.Sprint(s.KeyID) + "~" + hex.EncodeToString(s.Signature)
					psig = append(psig, cs)
				}

				esig := make([]string, 0)
				for _, s := range tx.EnvelopeSignatures {
					cs := hex.EncodeToString(s.Address[:]) + "~" + fmt.Sprint(s.SignerIndex) + "~" + fmt.Sprint(s.KeyID) + "~" + hex.EncodeToString(s.Signature)
					esig = append(esig, cs)
				}

				envelopeSize := 0
				envelopeSize += 32             // ReferenceBlockID
				envelopeSize += len(tx.Script) // Script
				for _, arg := range tx.Arguments {
					envelopeSize += len(arg) // arg size
				}
				envelopeSize += 8                          // GasLimit
				envelopeSize += flow.AddressLength + 8 + 8 // ProposalKey
				envelopeSize += flow.AddressLength         // Payer
				for _ = range tx.Authorizers {
					envelopeSize += flow.AddressLength // per authorizer
				}
				for _, sig := range tx.PayloadSignatures {
					envelopeSize += flow.AddressLength + 8 + 8 + len(sig.Signature) // per payload sig
				}
				for _, sig := range tx.EnvelopeSignatures {
					envelopeSize += flow.AddressLength + 8 + 8 + len(sig.Signature) // per env sig
				}

				// TODO
				// ErrorMessage                 string   `json:"error_message"`

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
					ProposalKeyID:             tx.ProposalKey.KeyID,
					ProposalSequenceNumber:    tx.ProposalKey.SequenceNumber,
					AuthorizersAddressHex:     auths,
					EnvelopeSize:              envelopeSize,
					CompactPayloadSignatures:  psig,
					CompactEnvelopeSignatures: esig,
				}

				// str := "{\"script\": \"" + hex.EncodeToString(tx.Script) + "\", " +
				// 	" \"tx_id\": \"" + hex.EncodeToString(txID[:]) + "\", " +
				// 	" \"col_id\": \"" + hex.EncodeToString(colID[:]) + "\", " +
				// 	" \"block_id\": \"" + hex.EncodeToString(activeBlockID[:]) + "\", " +
				// 	" \"col_index\": " + fmt.Sprint(i) + ", " +
				// 	" \"block_height\": " + fmt.Sprint(header.Height) + ", " +
				// 	" \"ref_block_id\": \"" + hex.EncodeToString(tx.ReferenceBlockID[:]) + "\" }\n"

				jsonData, err := json.Marshal(tic)
				if err != nil {
					log.Fatal().Err(err).Msg("could not create a json obj for a transaction")
				}
				str := string(jsonData)

				_, err = txWriter.WriteString(str)
				if err != nil {
					log.Fatal().Err(err).Msg("could not write transaction")
					done = true
				}
				txWriter.Flush()
			}
		}

		// TransactionBody
		// Status           TransactionStatus
		// Events           []Event
		// ComputationSpent uint64
		// StartState       StateCommitment
		// EndState         StateCommitment

		evs, err := events.ByBlockID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not fetch events")
		}
		for _, ev := range evs {
			str := "{\"event_type\": \"" + string(ev.Type) + "\", " +
				" \"tx_id\": \"" + hex.EncodeToString(ev.TransactionID[:]) + "\", " +
				" \"tx_index\": " + fmt.Sprint(ev.TransactionIndex) + ", " +
				" \"event_index\": " + fmt.Sprint(ev.EventIndex) + ", " +
				" \"payload\": \"" + hex.EncodeToString(ev.Payload) + "\" }\n"
			_, err := eventWriter.WriteString(str)
			if err != nil {
				log.Fatal().Err(err).Msg("could not fetch events")
				done = true
			}
			eventWriter.Flush()
		}

		activeBlockID = header.ParentID
	}

	// genesisState, err := commits.ByBlockID(genesis.ID())
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("could not load genesis state")
	// }

	// log.Info().Msgf("genesis state commitment %x empty state commitment %x", genesisState, emptyTrieRootHash)

	// step := 50_000
	// last := 1_065_711

	// megaMapping := make(map[string]delta.Mapping, 0)
	// for i := 0; i <= last; i += step {
	// 	end := i + step - 1
	// 	if end > last {
	// 		end = last
	// 	}
	// 	megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)
	// }

}
