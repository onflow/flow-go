package main

import (
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence/runtime"
	"github.com/r3labs/diff"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/fvm"
	state2 "github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/flattener"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

type Update struct {
	StateCommitment flow.StateCommitment
	Snapshot        *delta.Snapshot
}

type ComputedBlock struct {
	entity.ExecutableBlock
	Updates  []Update //collectionID -> update
	EndState flow.StateCommitment
	Results  []flow.TransactionResult
}

type Loader struct {
	headers          *badger.Headers
	index            *badger.Index
	identities       *badger.Identities
	guarantees       *badger.Guarantees
	seals            *badger.Seals
	payloads         *badger.Payloads
	commits          *badger.Commits
	transactions     *badger.Transactions
	collections      *badger.Collections
	executionResults *badger.ExecutionResults
	blocks           *badger.Blocks
	chunkDataPacks   *badger.ChunkDataPacks
	executionState   state.ExecutionState
	metrics          *metrics.NoopCollector
}

func main() {

	db := common.InitStorage("/Users/makspawlak/Downloads/candidate1-execution/protocol")
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	tracer := &trace.NoopTracer{}

	index := badger.NewIndex(cacheMetrics, db)
	identities := badger.NewIdentities(cacheMetrics, db)
	guarantees := badger.NewGuarantees(cacheMetrics, db)
	seals := badger.NewSeals(cacheMetrics, db)
	transactions := badger.NewTransactions(cacheMetrics, db)
	headers := badger.NewHeaders(cacheMetrics, db)

	commits := badger.NewCommits(cacheMetrics, db)
	payloads := badger.NewPayloads(db, index, identities, guarantees, seals)
	blocks := badger.NewBlocks(db, headers, payloads)
	collections := badger.NewCollections(db, transactions)
	chunkDataPacks := badger.NewChunkDataPacks(db)
	executionResults := badger.NewExecutionResults(db)
	executionState := state.NewExecutionState(nil, commits, blocks, collections, chunkDataPacks, executionResults, db, tracer)
	loader := Loader{
		headers:          headers,
		index:            index,
		identities:       identities,
		guarantees:       guarantees,
		seals:            seals,
		payloads:         payloads,
		commits:          commits,
		transactions:     transactions,
		collections:      collections,
		executionResults: executionResults,
		blocks:           blocks,
		chunkDataPacks:   chunkDataPacks,
		executionState:   executionState,
		metrics:          cacheMetrics,
	}

	genesis, err := blocks.ByHeight(0)
	if err != nil {
		log.Fatal().Err(err).Msg("could not load genesis")
	}
	genesisState, err := commits.ByBlockID(genesis.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("could not load genesis state")
	}

	emptyTrieRootHash := trie.EmptyTrieRootHash(ledger.RegisterKeySize)

	log.Info().Msgf("genesis state commitment %x empty state commitment %x", genesisState, emptyTrieRootHash)

	//step := 200_000
	step := 50_000
	//last := 1_065_711
	last := 49_999

	for i := 0; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		loader.ProcessBlocks(uint64(i), uint64(end))
	}

}

func dumpEntity(entity interface{}) {

	if e, ok := entity.(flow.Entity); ok {
		fmt.Printf("ID %s Checksum %s \n", e.ID().String(), e.Checksum().String())
	}

	spew.Dump(entity)

}

// retrieve block with collection and deltas and chunks to rebuild its interactions
// with ledger
func (l *Loader) addToList(header *flow.Header, m map[uint64][]*ComputedBlock) int {

	totalTransactions := 0

	payload, err := l.payloads.ByBlockID(header.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("could not get payload")
	}

	stateCommitment, err := l.commits.ByBlockID(header.ParentID)
	if err != nil {
		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("no state commitment found for block")
	}

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(payload.Guarantees))

	for _, collectionGuarantee := range payload.Guarantees {
		collection, err := l.collections.ByID(collectionGuarantee.CollectionID)
		if err != nil {
			log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("collection not found")
		}
		completeCollections[collectionGuarantee.CollectionID] = &entity.CompleteCollection{
			Guarantee:    collectionGuarantee,
			Transactions: collection.Transactions,
		}
		totalTransactions += len(collection.Transactions)
	}

	updates := make([]Update, len(completeCollections))

	delta, err := l.executionState.RetrieveStateDelta(context.Background(), header.ID())
	if err != nil {
		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot load delta")
	}

	executionResult, err := l.executionResults.ByBlockID(header.ID())
	if err != nil {
		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot load execution results")
	}
	if executionResult.BlockID != header.ID() {
		spew.Dump(header)
		spew.Dump(payload)
		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("execution result ID different from asked")
	}

	for i, chunk := range executionResult.Chunks {
		chunkDataPack, err := l.chunkDataPacks.ByChunkID(chunk.ID())
		if err != nil {
			log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot load chunk data pack")
		}
		if chunkDataPack.ID() != chunk.ID() {
			spew.Dump(header)
			spew.Dump(payload)
			log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("chunk data pack ID different from asked")
		}
		//if _, has := completeCollections[chunkDataPack.CollectionID]; !has {
		//	fmt.Println("Header")
		//	dumpEntity(header)
		//	fmt.Println("Payload")
		//	dumpEntity(payload)
		//	fmt.Println("Execution result")
		//	dumpEntity(executionResult)
		//	fmt.Println("Chunk data part")
		//	dumpEntity(chunkDataPack)
		//	fmt.Println("Complete collection")
		//	dumpEntity(completeCollections)
		//	//log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("chunk data pack collection ID not present in complete collections")
		//	log.Info().Str("block_id", header.ID().String()).Uint64("block_height", header.Height).Msg("chunk data pack collection ID not present in complete collections - skippping")
		//
		//	var alternativeER []*flow.ExecutionResult
		//
		//	err := l.executionResults.IterateResults(func(er *flow.ExecutionResult) bool {
		//		if er.BlockID == header.ID() {
		//			alternativeER = append(alternativeER, er)
		//		}
		//		return true
		//	})
		//	if err != nil {
		//		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot search for alternative ER")
		//	}
		//	fmt.Println("Alternative ERs")
		//	dumpEntity(alternativeER)
		//
		//	var alternativeCDP []*flow.ChunkDataPack
		//	err = l.chunkDataPacks.IterateChunkDataPacka(func(er *flow.ChunkDataPack) bool {
		//		if er.CollectionID == chunkDataPack.CollectionID {
		//			alternativeCDP = append(alternativeCDP, er)
		//		}
		//		return true
		//	})
		//	if err != nil {
		//		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot search for alternative CDP")
		//	}
		//	fmt.Println("Alternative CDP for offending collection")
		//	dumpEntity(alternativeCDP)
		//
		//	var anotherCDP []*flow.ChunkDataPack
		//	err = l.chunkDataPacks.IterateChunkDataPacka(func(er *flow.ChunkDataPack) bool {
		//		if _, has := completeCollections[er.CollectionID]; has {
		//			anotherCDP = append(anotherCDP, er)
		//		}
		//		return true
		//	})
		//	if err != nil {
		//		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot search for another CDP")
		//	}
		//	fmt.Println("Another CDP for offending collection")
		//	dumpEntity(anotherCDP)
		//
		//
		//
		//	collection, err := l.collections.ByID(chunkDataPack.CollectionID)
		//	if err != nil {
		//		log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot find non-existing entity")
		//	}
		//	fmt.Println("Offending collection")
		//	dumpEntity(collection)
		//
		//	log.Fatal().Msg("error")
		//
		//	return 0
		//} else {
		//	log.Info().Str("block_id", header.ID().String()).Uint64("block_height", header.Height).Msg("chunk data pack collection ID  present in complete collections - all good")
		//
		//}

		updates[i] = Update{
			StateCommitment: chunkDataPack.StartState,
			Snapshot:        delta.StateInteractions[i],
		}
	}

	m[header.Height] = append(m[header.Height], &ComputedBlock{
		ExecutableBlock: entity.ExecutableBlock{
			Block: &flow.Block{
				Header:  header,
				Payload: payload,
			},
			CompleteCollections: completeCollections,
			StartState:          stateCommitment,
		},
		EndState: delta.EndState,
		Updates:  updates,
		Results:  delta.TransactionResults,
	})

	return totalTransactions
}

func (l *Loader) findUpdates(blocks map[uint64][]*ComputedBlock) map[string]*trie.MTrie {

	hashes := make(map[string]*trie.MTrie)

	ledgerWAL, err := wal.NewWAL(nil, nil, "/Users/makspawlak/Downloads/candidate1-execution/execution", len(blocks), ledger.RegisterKeySize, wal.SegmentSize)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create WAL")
	}

	//gather all hashes
	for _, computedBlocks := range blocks {
		for _, computerBlock := range computedBlocks {
			hashes[string(computerBlock.EndState)] = nil
			hashes[string(computerBlock.StartState)] = nil
		}
	}

	mForest, err := mtrie.NewMForest(ledger.RegisterKeySize, "", len(blocks), l.metrics, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create mForest")
	}

	err = ledgerWAL.ReplayLogsOnly(
		func(_ *flattener.FlattenedForest) error {
			return fmt.Errorf("not expecting checkpoints")
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			mTrie, err := mForest.Update(stateCommitment, keys, values)
			if err != nil {
				return fmt.Errorf("cannot update trie")
			}
			newHash := mTrie.RootHash()
			hashes[string(newHash)] = mTrie

			return nil
		},
		func(_ flow.StateCommitment) error {
			return nil
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot replay WAL")
	}

	for _, mTrie := range hashes {
		if mTrie == nil {
			log.Fatal().Err(err).Msg("some state commitments not found")
		}
	}

	return hashes
}

func (l *Loader) ProcessBlocks(start uint64, end uint64) {

	var blockData map[uint64][]*ComputedBlock

	gob.Register(crypto.PubKeyBLSBLS12381{})

	blocksFilename := fmt.Sprintf("data/blocks_%d_%d.gob", start, end)
	if _, err := os.Stat(blocksFilename); !os.IsNotExist(err) && false {

		log.Info().Msg("Loading block data from file")

		//load file
		dataFile, err := os.Open(blocksFilename)
		if err != nil {
			log.Fatal().Err(err).Msg("could not open file for blocks")
		}
		defer dataFile.Close()

		// serialize the data
		dataEncoder := gob.NewDecoder(dataFile)

		err = dataEncoder.Decode(&blockData)
		if err != nil {
			log.Fatal().Err(err).Msg("could not decode blocks")
		}

		log.Info().Msg("Blocks data loaded")

	} else {

		log.Info().Msgf("Processing blocks from  %d to %d", start, end)

		blockData = make(map[uint64][]*ComputedBlock, end-start)

		rangeStart := start
		rangeStop := end

		totalTx := 0

		// abuse findHeader to get all the blocks
		_, err := l.headers.FindHeaders(func(header *flow.Header) bool {
			if header.Height >= rangeStart && header.Height <= rangeStop {
				totalTx += l.addToList(header, blockData)
			}

			return false
		})
		if err != nil {
			log.Fatal().Err(err).Msg("could not collect blocks")
		}

		log.Info().Msgf("Finished processing total of %d blocks containing %d transactions\n", len(blockData), totalTx)

		////save to file
		//dataFile, err := os.Create(blocksFilename)
		//if err != nil {
		//	log.Fatal().Err(err).Msg("could not create file for blocks")
		//}
		//defer dataFile.Close()
		//
		//// serialize the data
		//dataEncoder := gob.NewEncoder(dataFile)
		//
		//err = dataEncoder.Encode(blockData)
		//if err != nil {
		//	log.Fatal().Err(err).Msg("could not encode blocks")
		//}
	}

	updates := l.findUpdates(blockData)

	rt := runtime.NewInterpreterRuntime()

	vm := fvm.New(rt)

	for i := start; i <= end; i++ {

		blocks, has := blockData[i]

		if !has {
			log.Fatal().Msgf("Block height %d did not collect any blocks", i)
		}

		for _, computedBlock := range blocks {

			startState := computedBlock.StartState

			mTrie := updates[string(startState)]

			blockView := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
				read, err := mTrie.UnsafeRead([][]byte{state2.RegisterID(owner, controller, key)})
				if err != nil {
					return nil, err
				}
				return read[0], nil
			})

			type mapping struct {
				owner []byte
				key   []byte
			}

			vmCtx := fvm.NewContext(
				fvm.WithChain(flow.Mainnet.Chain()),
				fvm.WithBlocks(l.blocks),
			)

			computationManager := computation.New(
				zerolog.Nop(), l.metrics, nil,
				nil, //module.Local should not be used
				nil, //protocol.State should not be used
				vm, vmCtx,
			)

			computationResult, err := computationManager.ComputeBlock(
				context.Background(),
				&computedBlock.ExecutableBlock,
				blockView,
			)

			if err != nil {
				log.Fatal().Err(err).Str("block_id", computedBlock.ID().String()).Msg("cannot compute block")
			}

			if len(computedBlock.Block.Payload.Guarantees) > 0 {
				log.Info().Msgf("Block %000000d", computedBlock.Block.Header.Height)

				changelog, err := diff.Diff(computedBlock.Results, computationResult.TransactionResult)
				if err != nil {
					log.Fatal().Err(err).Str("block_id", computedBlock.ID().String()).Msg("cannot compare results")
				}
				fmt.Println("Tx results diff")
				spew.Dump(changelog)

				//	log.Info().Msg("Reads")
				//	spew.Dump(readMappings)
				//
				//	log.Info().Msg("Writes")
				//	spew.Dump(writeMappings)
			}

			for i, _ := range computedBlock.Block.Payload.Guarantees {
				//collectionID := collectionGuarantee.CollectionID

				calculatedSnapshot := computationResult.StateSnapshots[i]
				originalSnapshot := computedBlock.Updates[i].Snapshot

				if originalSnapshot == nil {
					log.Info().Msgf("Original snapshot %d does not exist", i)
					spew.Dump(computedBlock)
				}
				if calculatedSnapshot == nil {
					log.Info().Msgf("Calculated snapshot %d does not exist", i)
				}

				calculatedDelta := calculatedSnapshot.Delta
				originalDelta := originalSnapshot.Delta

				if !reflect.DeepEqual(calculatedDelta.Data, originalDelta.Data) {
					log.Info().Msg("snapshot dont match")

					//spew.Dump(originalDelta)
					//spew.Dump(calculatedDelta)

					changelog, err := diff.Diff(originalDelta.Data, calculatedDelta.Data)

					if err != nil {
						log.Fatal().Err(err).Str("block_id", computedBlock.ID().String()).Msg("cannot compare")
					}

					for _, change := range changelog {
						key := change.Path[0]

						fullKey, has := calculatedDelta.WriteMappings[key]

						if !has {
							fmt.Printf("Key not found in mapping - %s: %x => %s \n", change.Type, key, change.From)
						} else {
							fmt.Printf("Changed key - %s '%x' '%x' '%s' => %s \n", change.Type, fullKey.Owner, fullKey.Controller, fullKey.Key, change.From)
						}

					}

					//spew.Dump(changelog)
					//spew.Dump(writeMappings)
					//spew.Dump(readMappings)

				} else {
					log.Info().Msg("snapshot do   match")
				}
			}

		}
	}

}
