package export

import (
	"bufio"
	"encoding/hex"
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
	// index := badger.NewIndex(cacheMetrics, db)
	// identities := badger.NewIdentities(cacheMetrics, db)
	// guarantees := badger.NewGuarantees(cacheMetrics, db)
	// seals := badger.NewSeals(cacheMetrics, db)
	// transactions := badger.NewTransactions(cacheMetrics, db)
	headers := badger.NewHeaders(cacheMetrics, db)

	// commits := badger.NewCommits(cacheMetrics, db)
	// payloads := badger.NewPayloads(db, index, identities, guarantees, seals)
	// blocks := badger.NewBlocks(db, headers, payloads)
	// collections := badger.NewCollections(db, transactions)
	// chunkDataPacks := badger.NewChunkDataPacks(db)
	// executionResults := badger.NewExecutionResults(db)
	// executionState := state.NewExecutionState(nil, commits, blocks, collections, chunkDataPacks, executionResults, db, tracer)

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

	outputFile := outputPath + "/events.txt"
	fi, err := os.Create(outputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create output file")
	}
	defer fi.Close()
	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	for !done {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not load header")
			done = true
		}
		// block, err := blocks.ByID(activeBlockID)
		// if err != nil {
		// 	log.Fatal().Err(err).Msg("could not load block")
		// 	done = true
		// }

		evs, err := events.ByBlockID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not fetch events")
		}
		for _, ev := range evs {
			str := "{\"event_type\": \"" + string(ev.Type) + "\", " +
				" \"tx_id\": \"" + hex.EncodeToString(ev.TransactionID[:]) + "\", " +
				" \"tx_index\": " + string(ev.TransactionIndex) + ", " +
				" \"event_index\": " + string(ev.EventIndex) + ", " +
				" \"payload\": \"" + hex.EncodeToString(ev.Payload) + "\", " +
				"}\n"
			_, err := writer.WriteString(str)
			if err != nil {
				log.Fatal().Err(err).Msg("could not fetch events")
				done = true
			}
			writer.Flush()
			fmt.Println(str)
		}

		// activeBlockID = block.Header.ParentID
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
