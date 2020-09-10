package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence"
	"github.com/pbenner/threadpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vmihailenco/msgpack"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"

	initialRuntime "example.com/cadence-initial/runtime"
	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
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
	cadenceRuntime "github.com/onflow/cadence/runtime"

	readBadger "github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

type Update struct {
	StateCommitment flow.StateCommitment
	Snapshot        *delta.Snapshot
}

type ComputedBlock struct {
	ExecutableBlock entity.ExecutableBlock
	Updates         []Update //collectionID -> update
	EndState        flow.StateCommitment
	Results         []flow.TransactionResult
}

type Loader struct {
	db               *readBadger.DB
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
	vm               *fvm.VirtualMachine
	ctx              context.Context
	mappingMutex     sync.Mutex
	executionDir     string
	dataDir          string
	blockIDsDir      string
	blocksDir        string
	systemChunk      bool
}

func newLoader(protocolDir, executionDir, dataDir, blockIDsDir, blocksDir string, vm *fvm.VirtualMachine, systemChunk bool) *Loader {
	db := common.InitStorage(protocolDir)
	// defer db.Close()

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
		db:               db,
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
		vm:               vm,
		ctx:              context.Background(),
		executionDir:     executionDir,
		dataDir:          dataDir,
		blockIDsDir:      blockIDsDir,
		blocksDir:        blocksDir,
		systemChunk:      systemChunk,
	}
	return &loader
}

var debugStateCommitments = false

func main() {

	go func() {
		err := http.ListenAndServe(":4000", nil)
		log.Fatal().Err(err).Msg("pprof server error")
	}()

	//candidate1()
	//candidate4()
	//candidate5()
	//candidate6()
	candidate7()

}

func candidate1() {

	initialRT := initialRuntime.NewInterpreterRuntime()
	vm := fvm.NewWithInitial(initialRT)

	loader := newLoader("/home/m4ks/candidate1-execution/protocol/", "/home/m4ks/candidate1-execution/execution", "data1", "block-ids", "blocks", vm, false)
	defer loader.Close()

	genesis, err := loader.blocks.ByHeight(0)
	if err != nil {
		log.Fatal().Err(err).Msg("could not load genesis")
	}
	genesisState, err := loader.commits.ByBlockID(genesis.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("could not load genesis state")
	}

	emptyTrieRootHash := trie.EmptyTrieRootHash(ledger.RegisterKeySize)

	log.Info().Msgf("genesis state commitment %x empty state commitment %x", genesisState, emptyTrieRootHash)

	//step := 200_000
	step := 50_000
	last := 1_065_711
	//last := 49_999

	megaMapping := make(map[string]delta.Mapping, 0)

	for i := 0; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)
		runtime.GC()
	}
}

func candidate4() {

	initialRT := initialRuntime.NewInterpreterRuntime()
	vm := fvm.NewWithInitial(initialRT)

	loader := newLoader("/home/m4ks/candidate4-execution/protocol", "/home/m4ks/candidate4-execution/execution", "data4", "block-ids", "blocks", vm, false)

	step := 10_000
	first := 1_065_712
	last := 2_033_592

	megaMapping := readMegamappings("/home/m4ks/candidate4-execution/execution/root.checkpoint.mapping.json")

	for i := first; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)
		runtime.GC()
	}
}

func candidate5() {

	initialRT := initialRuntime.NewInterpreterRuntime()
	vm := fvm.NewWithInitial(initialRT)

	loader := newLoader("/home/m4ks/candidate5-execution/protocol", "/home/m4ks/candidate5-execution/execution", "data5", "block-ids", "blocks", vm, false)

	step := 50_000
	first := 2_033_593
	last := 3_187_931

	megaMapping := readMegamappings("/home/m4ks/candidate5-execution/execution/root.checkpoint.mapping.json")

	for i := first; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)

		runtime.GC()

	}
}

func candidate6() {

	initialRT := cadenceRuntime.NewInterpreterRuntime()
	vm := fvm.New(initialRT)

	loader := newLoader("/home/m4ks/candidate6-execution/protocol", "/home/m4ks/candidate6-execution/execution", "data6", "block-ids", "blocks", vm, false)

	step := 50_000
	first := 3_187_932
	last := 4_132_133

	megaMapping := readMegamappings("/home/m4ks/candidate6-execution/execution/root.checkpoint.mapping.json")

	for i := first; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)

		runtime.GC()

	}
}

func candidate7() {

	initialRT := cadenceRuntime.NewInterpreterRuntime()
	vm := fvm.New(initialRT)

	loader := newLoader("/home/m4ks/candidate7-execution/protocol", "/home/m4ks/candidate7-execution/execution", "data7", "block-ids", "blocks", vm, true)

	step := 50_000
	first := 4_132_134
	last := 4_972_987

	megaMapping := readMegamappings("/home/m4ks/candidate7-execution/execution/root.checkpoint.mapping.json")

	for i := first; i <= last; i += step {
		end := i + step - 1
		if end > last {
			end = last
		}
		megaMapping = loader.ProcessBlocks(uint64(i), uint64(end), megaMapping)

		runtime.GC()

	}
}

func dumpEntity(entity interface{}) {

	if e, ok := entity.(flow.Entity); ok {
		fmt.Printf("ID %s Checksum %s \n", e.ID().String(), e.Checksum().String())
	}

	spew.Dump(entity)

}

func (l *Loader) connectToMongo() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://maks:QvLfBqkibC9kogFdVN!VKA64@portal-ssl489-5.flow-temp-mongodb.3981023.composedb.com:19806,portal-ssl1127-2.flow-temp-mongodb.3981023.composedb.com:19806/compose?authSource=admin&ssl=true&retryWrites=false"))
	if err != nil {
		log.Fatal().Err(err)
	}

	ctx, _ := context.WithTimeout(l.ctx, 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal().Err(err)
	}

	return client
}

// retrieve block with collection and deltas and chunks to rebuild its interactions
// with ledger
func (l *Loader) addToList(header *flow.Header, m map[uint64][]*ComputedBlock) int {

	hexID := header.ID().String()

	fileDir := fmt.Sprintf("%s/%s/%s/%s", l.blocksDir, string(hexID[0]), string(hexID[1]), string(hexID[2]))
	filename := fmt.Sprintf("%s/%s.msgpack", fileDir, hexID)

	if _, err := os.Stat(filename); !os.IsNotExist(err) && false {

		var computedBlock ComputedBlock

		bytes, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot read blockIDs files")
		}
		err = msgpack.Unmarshal(bytes, &computedBlock)
		// err = json.Unmarshal(bytes, &heightToBlockIDs)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot unmarshall block msgpack")
		}

		m[header.Height] = append(m[header.Height], &computedBlock)

		totalTxs := 0

		for _, cc := range computedBlock.ExecutableBlock.CompleteCollections {
			totalTxs += len(cc.Transactions)
		}

		return totalTxs

	}

	totalTransactions := 0

	payload, err := l.payloads.ByBlockID(header.ID())
	if err != nil {
		log.Fatal().Err(err).Msg("could not get payload")
	}

	stateCommitment, err := l.commits.ByBlockID(header.ParentID)
	if err != nil {
		log.Warn().Err(err).Str("block_id", header.ID().String()).Uint64("height", header.Height).Str("parent_id", header.ParentID.String()).Msg("no state commitment found for block")
		return 0
	}

	if debugStateCommitments {
		log.Info().Hex("state_commitment", stateCommitment).Uint64("height", header.Height).Msg("state commitment for block")
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

	chunksCount := len(completeCollections)
	if l.systemChunk {
		chunksCount++
	}

	updates := make([]Update, chunksCount)

	delta, err := l.executionState.RetrieveStateDelta(context.Background(), header.ID())
	if err != nil {
		log.Warn().Err(err).Str("block_id", header.ID().String()).Uint64("height", header.Height).Msg("cannot load delta")
		return 0
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

	lastIndex := len(executionResult.Chunks)
	// if l.systemChunk {
	// 	lastIndex -= 1
	// }

	for i := 0; i < lastIndex; i++ {
		chunk := executionResult.Chunks[i]

		//	}

		//	for i, chunk := range executionResult.Chunks {
		chunkDataPack, err := l.chunkDataPacks.ByChunkID(chunk.ID())
		if err != nil {
			log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("cannot load chunk data pack")
		}
		if chunkDataPack.ID() != chunk.ID() {
			spew.Dump(header)
			spew.Dump(payload)
			log.Fatal().Err(err).Str("block_id", header.ID().String()).Msg("chunk data pack ID different from asked")
		}

		updates[i] = Update{
			StateCommitment: chunkDataPack.StartState,
			Snapshot:        delta.StateInteractions[i],
		}
	}

	computerBlock := &ComputedBlock{
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
	}

	m[header.Height] = append(m[header.Height], computerBlock)

	// megaJson, err := msgpack.Marshal(computerBlock)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("could not msgpack block")
	// }

	// err = os.MkdirAll(fileDir, os.ModePerm)
	// if err != nil {
	// 	log.Fatal().Err(err).Msgf("could not create block dir: %s", fileDir)
	// }

	// err = ioutil.WriteFile(filename, megaJson, 0644)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("could not write block file")
	// }

	return totalTransactions
}

func (l *Loader) findUpdates(blocks map[uint64][]*ComputedBlock) map[string]*trie.MTrie {

	gotAllHashes := fmt.Errorf("Got all hashes, bailing out fast")

	toFind := make(map[string]struct{})
	required := make(map[string]struct{})
	hashes := make(map[string]*trie.MTrie)

	ledgerWAL, err := wal.NewWAL(log.Logger, nil, l.executionDir, len(blocks), ledger.RegisterKeySize, wal.SegmentSize)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create WAL")
	}

	//gather all hashes
	for _, computedBlocks := range blocks {
		for _, computerBlock := range computedBlocks {
			hashes[string(computerBlock.EndState)] = nil
			hashes[string(computerBlock.ExecutableBlock.StartState)] = nil

			toFind[string(computerBlock.EndState)] = struct{}{}
			toFind[string(computerBlock.ExecutableBlock.StartState)] = struct{}{}

			required[string(computerBlock.EndState)] = struct{}{}
			required[string(computerBlock.ExecutableBlock.StartState)] = struct{}{}
		}
	}

	mForest, err := mtrie.NewMForest(ledger.RegisterKeySize, "", 1000, l.metrics, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create mForest")
	}

	err = ledgerWAL.ReplayLogsOnly(
		func(ff *flattener.FlattenedForest) error {

			rebuiltTries, err := flattener.RebuildTries(ff)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}

			for _, mTrie := range rebuiltTries {
				newHash := string(mTrie.RootHash())

				if debugStateCommitments {
					log.Info().Str("root_hash", mTrie.StringRootHash()).Msg("Loaded from checkpoint")
				}

				hashes[string(newHash)] = mTrie

				delete(toFind, string(newHash))

				err := mForest.AddTrie(mTrie)
				if err != nil {
					return fmt.Errorf("cannot add trie while rebuilding forest: %w", err)
				}

				if len(toFind) == 0 {
					return gotAllHashes
				}

			}

			return nil
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			mTrie, err := mForest.Update(stateCommitment, keys, values)
			if err != nil {
				return fmt.Errorf("cannot update trie: %w", err)
			}
			err = mForest.AddTrie(mTrie)
			if err != nil {
				return fmt.Errorf("cannot add trie while updating WAL: %w", err)
			}
			newHash := string(mTrie.RootHash())

			if debugStateCommitments {
				log.Info().Str("root_hash", mTrie.StringRootHash()).Msg("Loaded from WAL")
			}

			hashes[string(newHash)] = mTrie

			delete(toFind, string(newHash))

			if len(toFind) == 0 {
				return gotAllHashes
			}

			return nil
		},
		func(_ flow.StateCommitment) error {
			return nil
		},
	)
	if err != nil && !errors.Is(err, gotAllHashes) {
		log.Fatal().Err(err).Msg("cannot replay WAL")
	}

	hasMissing := false

	for sc, mTrie := range hashes {
		if mTrie == nil {
			log.Info().Hex("state_commitment", []byte(sc)).Msg("some state commitments not found")
			hasMissing = true
		} else {
			newHash := string(mTrie.RootHash())

			if _, has := required[newHash]; !has {
				delete(hashes, newHash) //conserve memory I guess
			}
		}
	}

	if hasMissing {
		log.Fatal().Msg("Could not load all tries")

	}

	log.Info().Msgf("Finished loading %d tries", len(hashes))

	return hashes
}

func readMegamappings(filename string) map[string]delta.Mapping {
	var readMappings = map[string]delta.Mapping{}
	var hexencodedRead map[string]delta.Mapping

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read files")
	}
	err = json.Unmarshal(bytes, &hexencodedRead)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot unmarshall megamapping")
	}

	for k, mapping := range hexencodedRead {
		decodeString, err := hex.DecodeString(k)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot decode key")
		}
		readMappings[string(decodeString)] = mapping
	}

	return readMappings
}

func (l *Loader) ProcessBlocks(start uint64, end uint64, prevMapping map[string]delta.Mapping) map[string]delta.Mapping {

	var blockData map[uint64][]*ComputedBlock

	log.Info().Msgf("Processing blocks from  %d to %d", start, end)

	blockData = make(map[uint64][]*ComputedBlock, end-start)

	rangeStart := start
	rangeStop := end

	totalTx := 0

	var megaMapping map[string]delta.Mapping

	filename := fmt.Sprintf("%s/megamapping_%d_%d.json", l.dataDir, rangeStart, rangeStop)

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		log.Info().Msgf("Loaded mappings %d to %d from megamappings file", start, end)
		return readMegamappings(filename)
	}

	if rangeStart == 0 {
		megaMapping = l.BootstrapMegamapping()
	} else {
		megaMapping = prevMapping
	}

	blockIDsFilename := fmt.Sprintf("%s/height_to_ids_g_%d_%d.json", l.blockIDsDir, rangeStart, rangeStop)
	if _, err := os.Stat(blockIDsFilename); !os.IsNotExist(err) {

		var heightToBlockIDs map[uint64][]string

		bytes, err := ioutil.ReadFile(blockIDsFilename)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot read blockIDs files")
		}
		err = json.Unmarshal(bytes, &heightToBlockIDs)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot unmarshall heighttoBlockID")
		}

		for i := rangeStart; i <= rangeStop; i++ {
			for _, hexID := range heightToBlockIDs[i] {
				id, err := flow.HexStringToIdentifier(hexID)
				if err != nil {
					log.Fatal().Err(err).Msg("cannot decode hexID")
				}
				header, err := l.headers.ByBlockID(id)
				if err != nil {
					log.Fatal().Err(err).Msg("cannot load header")
				}
				totalTx += l.addToList(header, blockData)
			}
		}

		log.Info().Msgf("Finished reading total of %d blocks containing %d transactions using cached height mappings", len(blockData), totalTx)
	} else {
		// abuse findHeader to get all the blocks
		_, err := l.headers.FindHeaders(func(header *flow.Header) bool {
			cpy := header
			if header.Height >= rangeStart && header.Height <= rangeStop {
				totalTx += l.addToList(cpy, blockData)
			}

			return false
		})
		if err != nil {
			log.Fatal().Err(err).Msg("could not collect blocks")
		}

		heightToBlockIDs := make(map[uint64][]string, len(blockData))
		for height, blocks := range blockData {
			ids := make([]string, len(blocks))

			for i, block := range blocks {
				ids[i] = block.ExecutableBlock.ID().String()
			}

			heightToBlockIDs[height] = ids
		}

		megaJson, _ := json.MarshalIndent(heightToBlockIDs, "", "  ")
		err = ioutil.WriteFile(blockIDsFilename, megaJson, 0644)
		if err != nil {
			log.Fatal().Err(err).Msg("could not write height to block")
		}

		log.Info().Msgf("Finished reading %d blocks containing %d transactions from protocol database", len(blockData), totalTx)

	}

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

	updates := l.findUpdates(blockData)

	//rt := runtime.NewInterpreterRuntime()

	type Output struct {
		BlockID              string `json:"block_id"`
		BlockHeight          uint64 `json:"block_height"`
		PrevBlockId          string `json:"prev_block_id"`
		CollectionIndex      int    `json:"collection_index"`
		CollectionID         string `json:"collection_id"`
		TxHash               string `json:"tx_hash"`
		StartStateCommitment string `json:"start_state_commitment"`
		Transaction          string `json:"transaction"`
	}

	//outputs := make([]Output, 0)

	//mongoClient := l.connectToMongo()
	//defer mongoClient.Disconnect(l.ctx)

	//collection := mongoClient.Database("ftx").Collection("complete_blocks")

	//bufferSize := 200
	//blocksBuffer := make([]interfa	//for _, block := range blockData[5449] {
	//	//	fmt.Printf("Block %d ID %x\n", block.Block.Header.Height, block.Block.Header.ID())
	//	//	spew.Dump(block)
	//	//}
	//	//
	//	//os.Exit(1)ce{}, 0, bufferSize+1)

	pool := threadpool.New(16, int(end-start)*2)
	g := pool.NewJobGroup()

	allBlocks := make([]*ComputedBlock, 0, int(end-start))

	for i := start; i <= end; i++ {

		////if i >= 163 && i <= 173 || (i == 0) {
		//if i == 5449 {
		//	// BLock with height
		//	for _, block := range blockData[i] {
		//		fmt.Printf("Block %d ID %x\n", block.Block.Header.Height, block.Block.Header.ID())
		//		spew.Dump(block)
		//	}
		//}

		blocks, has := blockData[i]

		if !has {
			log.Fatal().Msgf("Block height %d did not collect any blocks", i)
		}

		for _, computedBlock := range blocks {
			if !bytes.Equal(computedBlock.EndState, computedBlock.ExecutableBlock.StartState) {
				cb := computedBlock
				allBlocks = append(allBlocks, cb)
			}

			// cb := computedBlock

			// pool.AddJob(g, func(pool threadpool.ThreadPool, erf func() error) error {

			// 	blockMapping := l.executeBlock(cb, updates)

			// 	l.mappingMutex.Lock()
			// 	for k, v := range blockMapping {
			// 		megaMapping[k] = v
			// 	}
			// 	l.mappingMutex.Unlock()

			// 	return nil
			// })

		}
	}

	// blockResults := make([]map[string]delta.Mapping, len(allBlocks))

	pool.AddRangeJob(0, len(allBlocks), g, func(i int, pool threadpool.ThreadPool, erf func() error) error {
		blockMapping := l.executeBlock(allBlocks[i], updates)

		//blockResults[i] = blockMapping

		l.mappingMutex.Lock()
		defer l.mappingMutex.Unlock()

		for k, v := range blockMapping {
			megaMapping[k] = v
		}

		// remove already run blocks to reduce memory usage
		height := allBlocks[i].ExecutableBlock.Height()

		if height != end {
			to_remove := -1
			for ii, block := range blockData[height] {
				if block.ExecutableBlock.ID() == allBlocks[i].ExecutableBlock.ID() {
					to_remove = ii
					break
				}
			}
			if to_remove == -1 {
				log.Fatal().Msg("block not found in blockData")
			}

			blockData[height] = append(blockData[height][:to_remove], blockData[height][to_remove+1:]...)

			if len(blockData[height]) == 0 {
				delete(blockData, height)
			}
		}

		allBlocks[i] = nil

		return nil
	})

	pool.Wait(g)
	pool.Stop()
	// for _, r := range blockResults {
	// 	// l.mappingMutex.Lock()
	// 	for k, v := range r {
	// 		megaMapping[k] = v
	// 	}
	// 	// l.mappingMutex.Unlock()
	// 	//return nil
	// }

	for _, computedBlock := range blockData[end] {
		allMappingsFound := true

		lastTrie := updates[string(computedBlock.EndState)]

		iterator := flattener.NewNodeIterator(lastTrie)

		for iterator.Next() {
			node := iterator.Value()

			key := node.Key()
			if key != nil {
				if _, has := megaMapping[string(key)]; !has {
					allMappingsFound = false
					log.Warn().Hex("key", key).Msg("Megamapping missing")
					spew.Dump(node.Value())
				}
			}
		}

		if allMappingsFound {
			log.Info().Uint64("block", end).Msg("All mapping for trie at this height found")

			hexencodedMappings := make(map[string]delta.Mapping, len(megaMapping))
			for k, mapping := range megaMapping {
				hexencodedMappings[hex.EncodeToString([]byte(k))] = mapping
			}

			megaJson, _ := json.MarshalIndent(hexencodedMappings, "", "  ")
			err := ioutil.WriteFile(filename, megaJson, 0644)
			if err != nil {
				log.Fatal().Err(err).Msg("cannot write megamapping file")
			}

			var readMappings = map[string]delta.Mapping{}
			var hexencodedRead map[string]delta.Mapping
			// sanity check
			bytes, err := ioutil.ReadFile(filename)
			if err != nil {
				log.Fatal().Err(err).Msg("cannot read files")
			}
			err = json.Unmarshal(bytes, &hexencodedRead)
			if err != nil {
				log.Fatal().Err(err).Msg("cannot unmarshall megamapping while checking")
			}

			for k, mapping := range hexencodedRead {
				decodeString, err := hex.DecodeString(k)
				if err != nil {
					log.Fatal().Err(err).Msg("cannot decode key")
				}
				readMappings[string(decodeString)] = mapping
			}

			if !reflect.DeepEqual(megaMapping, readMappings) {
				spew.Dump(megaMapping)
				spew.Dump(readMappings)

				log.Fatal().Msg("json bad")
			}

		} else {
			log.Info().Uint64("block", end).Msg("Mapping missing at this height")
		}
	}

	return megaMapping
}

func (l *Loader) executeBlock(computedBlock *ComputedBlock, updates map[string]*trie.MTrie) map[string]delta.Mapping {

	writtenMappings := make(map[string]delta.Mapping, 0)

	startState := computedBlock.ExecutableBlock.StartState

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
		l.vm, vmCtx, l.systemChunk,
	)

	computationResult, err := computationManager.ComputeBlock(
		context.Background(),
		&computedBlock.ExecutableBlock,
		blockView,
	)

	if err != nil {
		log.Fatal().Err(err).Str("block_id", computedBlock.ExecutableBlock.ID().String()).Msg("cannot compute block")
	}

	txResultEqual := false

	if len(computedBlock.ExecutableBlock.Block.Payload.Guarantees) > 0 {
		//log.Info().Msgf("Block %000000d", computedBlock.ExecutableBlock.Block.Header.Height)

		//results := collapseByTxID(computedBlock.Results)

		if compareResults(computedBlock.Results, computationResult.TransactionResult) {
			//log.Info().Uint64("block_heights", computedBlock.ExecutableBlock.Block.Header.Height).Msg("Tx results equal!")
			txResultEqual = true
		} else {
			//log.Info().Uint64("block_heights", computedBlock.ExecutableBlock.Block.Header.Height).Msg("Tx results differ!")

			dir := fmt.Sprintf("failed-txs/%07d", computedBlock.ExecutableBlock.Height())

			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				log.Fatal().Err(err).Msg("cannot create dir for failed tx")
			}

			f, err := os.Create(fmt.Sprintf("%s/%s", dir, computedBlock.ExecutableBlock.ID()))
			if err != nil {
				log.Fatal().Err(err).Msg("cannot create dir for failed tx")
			}
			w := bufio.NewWriter(f)

			fmt.Fprintln(w, "DB results")

			spew.Fdump(w, computedBlock.Results)
			fmt.Fprintln(w, "Computer results")
			spew.Fdump(w, computationResult.TransactionResult)

			fmt.Fprintln(w, "---")
			spew.Fdump(w, computedBlock.ExecutableBlock.Block)

			//
			//fmt.Printf("Block time %s\n", computedBlock.ExecutableBlock.Block.Header.Timestamp.String())

			for i, guarantee := range computedBlock.ExecutableBlock.Block.Payload.Guarantees {
				fmt.Fprintf(w, "collection %d id %s\n", i, guarantee.CollectionID.String())
				for _, tx := range computedBlock.ExecutableBlock.CompleteCollections[guarantee.CollectionID].Transactions {
					fmt.Fprintf(w, "tx %s\n", tx.ID().String())
					//fmt.Println(string(tx.Script))
					spew.Fdump(w, tx)
				}
			}

			err = w.Flush()
			if err != nil {
				log.Fatal().Err(err).Msg("cannot flush failed tx file")
			}
			err = f.Close()
			if err != nil {
				log.Fatal().Err(err).Msg("cannot close failed tx file")
			}

		}

		//	log.Info().Msg("Reads")
		//	spew.Dump(readMappings)
		//
		//	log.Info().Msg("Writes")
		//	spew.Dump(writeMappings)
	}
	failed := false

	for i, _ := range computedBlock.ExecutableBlock.Block.Payload.Guarantees {
		//collectionID := collectionGuarantee.CollectionID

		calculatedSnapshot := computationResult.StateSnapshots[i]
		originalSnapshot := computedBlock.Updates[i].Snapshot

		if originalSnapshot == nil {
			log.Fatal().Msgf("Original snapshot %d does not exist", i)
			spew.Dump(computedBlock)
		}
		if calculatedSnapshot == nil {
			log.Fatal().Msgf("Calculated snapshot %d does not exist", i)
		}

		calculatedDelta := calculatedSnapshot.Delta
		originalDelta := originalSnapshot.Delta

		for key, val := range originalDelta.Data {
			if mapping, has := calculatedDelta.WriteMappings[key]; !has {
				failed = true
				log.Warn().Hex("key", []byte(key)).Msg("Key not found in mapping")
				spew.Dump(val)
			} else {
				writtenMappings[key] = mapping
			}
		}

		//if !reflect.DeepEqual(calculatedDelta.Data, originalDelta.Data) {
		//	log.Info().Msg("snapshot dont match")
		//
		//	//spew.Dump(originalDelta)
		//	//spew.Dump(calculatedDelta)
		//
		//	changelog, err := diff.Diff(originalDelta.Data, calculatedDelta.Data)
		//
		//	if err != nil {
		//		log.Fatal().Err(err).Str("block_id", computedBlock.ExecutableBlock.ID().String()).Msg("cannot compare")
		//	}
		//
		//	for _, change := range changelog {
		//		key := change.Path[0]
		//
		//		fullKey, has := calculatedDelta.WriteMappings[key]
		//
		//		if !has {
		//			fmt.Printf("Key not found in mapping - %s: %x => %s \n", change.Type, key, change.From)
		//		} else {
		//			fmt.Printf("Changed key - %s '%x' '%x' '%s' # %s => %s  \n", change.Type, fullKey.Owner, fullKey.Controller, fullKey.Key, change.From, change.To)
		//		}
		//	}
		//
		//	//if len(changelog) > 1 {
		//	//	for _, c := range computedBlock.CompleteCollections {
		//	//		for _, tx := range c.Transactions {
		//	//			fmt.Println("tx in block")
		//	//			fmt.Println(string(tx.Script))
		//	//		}
		//	//	}
		//	//}
		//
		//	//spew.Dump(changelog)
		//	//spew.Dump(writeMappings)
		//	//spew.Dump(readMappings)
		//	//return
		//} else {
		//	log.Info().Msg("snapshot do   match")
		//}

		msg := ""
		blockLogger := log.Info().Uint64("block", computedBlock.ExecutableBlock.Block.Header.Height).Int("collection_guarantee", i)

		if txResultEqual {
			msg = "equal"
		} else {
			msg = "not equal"
			blockLogger = blockLogger.Str("block_id", computedBlock.ExecutableBlock.ID().String())
		}

		if !failed {
			if !txResultEqual {
				blockLogger.Msgf("All mapping path found, txResult %s", msg)
			}
		} else {
			blockLogger.Msgf("Some paths missing, txResult %s", msg)

			fmt.Println("Original results")
			spew.Dump(computedBlock.Results)
			fmt.Println("Computed results")
			spew.Dump(computationResult.TransactionResult)
			fmt.Println("Transactions")
			spew.Dump(computedBlock.ExecutableBlock.CompleteCollections)
			fmt.Println("Transaction script")
			for _, cc := range computedBlock.ExecutableBlock.CompleteCollections {
				for _, tx := range cc.Transactions {
					fmt.Println(string(tx.Script))
				}
			}
			fmt.Println("/Transaction script")

			spew.Dump(originalDelta.Data)

			for key, _ := range originalDelta.Data {

				fmt.Printf("Original delta data key %x\n", key)

			}

			spew.Dump(calculatedDelta.Data)

			for key, _ := range calculatedDelta.Data {

				fmt.Printf("Calculated delta data key %x\n", key)

			}

			// if mapping, has := calculatedDelta.WriteMappings[key]; !has {
			// 	failed = true
			// 	log.Warn().Hex("key", []byte(key)).Msg("Key not found in mapping")
			// 	spew.Dump(val)
			// } else {
			// 	writtenMappings[key] = mapping
			// }

			os.Exit(1)
		}
	}

	return writtenMappings
}

func (l *Loader) BootstrapMegamapping() map[string]delta.Mapping {
	megaMapping := make(map[string]delta.Mapping, 100)

	bootstrapper := bootstrap.NewBootstrapper(log.Logger)

	bootstrapView := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return nil, nil
	})

	privateKey, err := testutil.GenerateAccountPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot generate pk")
	}

	fix64, err := cadence.NewUFix64("1234.0")
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create ufix64")
	}

	err = bootstrapper.BoostrapView(bootstrapView, privateKey.PublicKey(1000), fix64, flow.Mainnet.Chain())
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot bootstrap")
	}

	for k, v := range bootstrapView.Delta().WriteMappings {
		megaMapping[k] = v
	}

	return megaMapping
}

func (l *Loader) Close() {
	l.db.Close()
}

// func collapseByTxID(results []flow.TransactionResult) []flow.TransactionResult {
// 	if len(results) == 0 {
// 		return results
// 	}

// 	ret := make([]flow.TransactionResult, 0)

// 	prev := results[0]
// 	for i := 1; i < len(results); i++ {
// 		if prev != results[i] {
// 			ret = append(ret, prev)
// 			prev = results[i]
// 		}
// 	}
// 	ret = append(ret, prev)

// 	return ret
// }

func compareResults(a []flow.TransactionResult, b []flow.TransactionResult) bool {
	if len(a) != len(b) {
		return false
	}

	am := make(map[string]string)
	bm := make(map[string]string)

	for _, t := range a {
		am[t.TransactionID.String()] = t.ErrorMessage
	}
	for _, t := range b {
		bm[t.TransactionID.String()] = t.ErrorMessage
	}

	return reflect.DeepEqual(am, bm)
}
