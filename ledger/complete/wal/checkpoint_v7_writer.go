package wal

import (
	"encoding/hex"
	"fmt"
	"io"
	"path"

	"github.com/docker/go-units"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// StoreCheckpointV7SingleThread stores checkpoint file in v7 (payloadless) format in a single threaded manner.
func StoreCheckpointV7SingleThread(tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV7(tries, outputDir, outputFile, logger, 1)
}

// StoreCheckpointV7Concurrently stores checkpoint file in v7 (payloadless) format with max workers.
func StoreCheckpointV7Concurrently(tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV7(tries, outputDir, outputFile, logger, 16)
}

// StoreCheckpointV7 stores checkpoint file into a main file and 17 file parts using V7 format.
// V7 format is identical to V6 in structure but indicates the checkpoint contains payloadless tries.
//
// nWorker specifies how many workers to encode subtrie concurrently, valid range [1,16]
func StoreCheckpointV7(
	tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint) error {
	err := storeCheckpointV7(tries, outputDir, outputFile, logger, nWorker)
	if err != nil {
		cleanupErr := deleteCheckpointFiles(outputDir, outputFile)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup temp file %s, after running into error: %w", cleanupErr, err)
		}
		return err
	}

	return nil
}

func storeCheckpointV7(
	tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint) error {
	if len(tries) == 0 {
		logger.Info().Msg("no tries to be checkpointed")
		return nil
	}

	first, last := tries[0], tries[len(tries)-1]
	lg := logger.With().
		Int("version", 7).
		Bool("payloadless", true).
		Int("trie_count", len(tries)).
		Str("checkpoint_file", path.Join(outputDir, outputFile)).
		Logger()

	lg.Info().
		Str("first_hash", first.RootHash().String()).
		Uint64("first_reg_count", first.AllocatedRegCount()).
		Str("first_reg_size", units.BytesSize(float64(first.AllocatedRegSize()))).
		Str("last_hash", last.RootHash().String()).
		Uint64("last_reg_count", last.AllocatedRegCount()).
		Str("last_reg_size", units.BytesSize(float64(last.AllocatedRegSize()))).
		Msg("storing payloadless checkpoint")

	// make sure a checkpoint file with same name doesn't exist
	matched, err := findCheckpointPartFiles(outputDir, outputFile)
	if err != nil {
		return fmt.Errorf("fail to check if checkpoint file already exist: %w", err)
	}

	if len(matched) != 0 {
		return fmt.Errorf("checkpoint part file already exists: %v", matched)
	}

	subtrieRoots := createSubTrieRoots(tries)

	subTrieRootIndices, subTriesNodeCount, subTrieChecksums, err := storeSubTrieConcurrentlyV7(
		subtrieRoots,
		estimateSubtrieNodeCount(last),
		subTrieRootAndTopLevelTrieCount(tries),
		outputDir,
		outputFile,
		lg,
		nWorker,
	)
	if err != nil {
		return fmt.Errorf("could not store sub trie: %w", err)
	}

	lg.Info().Msgf("subtrie have been stored. sub trie node count: %v", subTriesNodeCount)

	topTrieChecksum, err := storeTopLevelNodesAndTrieRootsV7(
		tries, subTrieRootIndices, subTriesNodeCount, outputDir, outputFile, lg)
	if err != nil {
		return fmt.Errorf("could not store top level tries: %w", err)
	}

	err = storeCheckpointHeaderV7(subTrieChecksums, topTrieChecksum, outputDir, outputFile, lg)
	if err != nil {
		return fmt.Errorf("could not store checkpoint header: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).Msg("payloadless checkpoint file has been successfully stored")

	return nil
}

func storeCheckpointHeaderV7(
	subTrieChecksums []uint32,
	topTrieChecksum uint32,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	errToReturn error,
) {
	if len(subTrieChecksums) != subtrieCountByLevel(subtrieLevel) {
		return fmt.Errorf("expect subtrie level %v to have %v checksums, but got %v",
			subtrieLevel, subtrieCountByLevel(subtrieLevel), len(subTrieChecksums))
	}

	closable, err := createWriterForCheckpointHeader(outputDir, outputFile, logger)
	if err != nil {
		return fmt.Errorf("could not store checkpoint header: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)

	// write version - use V7 instead of V6
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointHeader, VersionV7))
	if err != nil {
		return fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	_, err = writer.Write(encodeSubtrieCount(subtrieCount))
	if err != nil {
		return fmt.Errorf("cannot write subtrie level into checkpoint header: %w", err)
	}

	for i, subtrieSum := range subTrieChecksums {
		_, err = writer.Write(encodeCRC32Sum(subtrieSum))
		if err != nil {
			return fmt.Errorf("cannot write %v-th subtriechecksum into checkpoint header: %w", i, err)
		}
	}

	_, err = writer.Write(encodeCRC32Sum(topTrieChecksum))
	if err != nil {
		return fmt.Errorf("cannot write top level trie checksum into checkpoint header: %w", err)
	}

	checksum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(checksum))
	if err != nil {
		return fmt.Errorf("cannot write CRC32 checksum to checkpoint header: %w", err)
	}
	return nil
}

func storeTopLevelNodesAndTrieRootsV7(
	tries []*trie.MTrie,
	subTrieRootIndices map[*node.Node]uint64,
	subTriesNodeCount uint64,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	checksumOfTopTriePartFile uint32,
	errToReturn error,
) {
	closable, err := createWriterForTopTries(outputDir, outputFile, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)

	// write version - use V7 instead of V6
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV7))
	if err != nil {
		return 0, fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	_, err = writer.Write(encodeNodeCount(subTriesNodeCount))
	if err != nil {
		return 0, fmt.Errorf("could not write subtrie node count: %w", err)
	}

	scratch := make([]byte, 1024*4)

	topLevelNodeIndices, topLevelNodesCount, err := storeTopLevelNodes(
		scratch,
		tries,
		subTrieRootIndices,
		subTriesNodeCount+1,
		writer)

	if err != nil {
		return 0, fmt.Errorf("could not store top level nodes: %w", err)
	}

	logger.Info().Msgf("top level nodes have been stored. top level node count: %v", topLevelNodesCount)

	err = storeTries(scratch, tries, topLevelNodeIndices, writer)
	if err != nil {
		return 0, fmt.Errorf("could not store trie root nodes: %w", err)
	}

	checksum, err := storeTopLevelTrieFooter(topLevelNodesCount, uint16(len(tries)), writer)
	if err != nil {
		return 0, fmt.Errorf("could not store footer: %w", err)
	}

	return checksum, nil
}

func storeSubTrieConcurrentlyV7(
	subtrieRoots [subtrieCount][]*node.Node,
	estimatedSubtrieNodeCount int,
	subAndTopNodeCount int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
	nWorker uint,
) (
	map[*node.Node]uint64,
	uint64,
	[]uint32,
	error,
) {
	logger.Info().Msgf("storing %v subtrie groups (v7) with average node count %v for each subtrie", subtrieCount, estimatedSubtrieNodeCount)

	if nWorker == 0 || nWorker > subtrieCount {
		return nil, 0, nil, fmt.Errorf("invalid nWorker %v, the valid range is [1,%v]", nWorker, subtrieCount)
	}

	jobs := make(chan jobStoreSubTrie, len(subtrieRoots))
	resultChs := make([]<-chan *resultStoringSubTrie, len(subtrieRoots))

	for i, roots := range subtrieRoots {
		resultCh := make(chan *resultStoringSubTrie)
		resultChs[i] = resultCh
		jobs <- jobStoreSubTrie{
			Index:  i,
			Roots:  roots,
			Result: resultCh,
		}
	}
	close(jobs)

	for i := 0; i < int(nWorker); i++ {
		go func() {
			for job := range jobs {
				roots, nodeCount, checksum, err := storeCheckpointSubTrieV7(
					job.Index, job.Roots, estimatedSubtrieNodeCount, outputDir, outputFile, logger)

				job.Result <- &resultStoringSubTrie{
					Index:     job.Index,
					Roots:     roots,
					NodeCount: nodeCount,
					Checksum:  checksum,
					Err:       err,
				}
				close(job.Result)
			}
		}()
	}

	results := make(map[*node.Node]uint64, subAndTopNodeCount)
	results[nil] = 0
	nodeCounter := uint64(0)
	checksums := make([]uint32, 0, len(subtrieRoots))

	for _, resultCh := range resultChs {
		result := <-resultCh

		if result.Err != nil {
			return nil, 0, nil, fmt.Errorf("fail to store %v-th subtrie, trie: %w", result.Index, result.Err)
		}

		for root, index := range result.Roots {
			if root == nil {
				results[root] = 0
			} else {
				results[root] = index + nodeCounter
			}
		}
		nodeCounter += result.NodeCount
		checksums = append(checksums, result.Checksum)
	}

	return results, nodeCounter, checksums, nil
}

func storeCheckpointSubTrieV7(
	i int,
	roots []*node.Node,
	estimatedSubtrieNodeCount int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	rootNodesOfAllSubtries map[*node.Node]uint64,
	totalSubtrieNodeCount uint64,
	checksumOfSubtriePartfile uint32,
	errToReturn error,
) {
	closable, err := createWriterForSubtrie(outputDir, outputFile, logger, i)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not create writer for sub trie: %w", err)
	}

	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)

	// write version - use V7 instead of V6
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV7))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write version into checkpoint subtrie file: %w", err)
	}

	subtrieRootNodes := make(map[*node.Node]uint64, len(roots))
	nodeCounter := uint64(1)

	logging := logProgress(fmt.Sprintf("storing %v-th sub trie roots (v7)", i), estimatedSubtrieNodeCount, logger)

	traversedSubtrieNodes := make(map[*node.Node]uint64, estimatedSubtrieNodeCount)
	traversedSubtrieNodes[nil] = 0

	scratch := make([]byte, 1024*4)
	for _, root := range roots {
		nodeCounter, err = storeUniqueNodes(root, traversedSubtrieNodes, nodeCounter, scratch, writer, logging)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("fail to store nodes in step 1 for subtrie root %v: %w", root.Hash(), err)
		}
		subtrieRootNodes[root] = traversedSubtrieNodes[root]
	}

	totalNodeCount := nodeCounter - 1

	checksum, err := storeSubtrieFooter(totalNodeCount, writer)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not store subtrie footer %w", err)
	}

	return subtrieRootNodes, totalNodeCount, checksum, nil
}

// storeUniqueNodesV7 is similar to storeUniqueNodes but logs with V7 context.
func storeUniqueNodesV7(
	root *node.Node,
	visitedNodes map[*node.Node]uint64,
	nodeCounter uint64,
	scratch []byte,
	writer io.Writer,
	nodeCounterUpdated func(nodeCounter uint64),
) (uint64, error) {
	for itr := flattener.NewUniqueNodeIterator(root, visitedNodes); itr.Next(); {
		n := itr.Value()

		visitedNodes[n] = nodeCounter
		nodeCounter++
		nodeCounterUpdated(nodeCounter)

		var lchildIndex, rchildIndex uint64

		if lchild := n.LeftChild(); lchild != nil {
			var found bool
			lchildIndex, found = visitedNodes[lchild]
			if !found {
				hash := lchild.Hash()
				return 0, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
			}
		}
		if rchild := n.RightChild(); rchild != nil {
			var found bool
			rchildIndex, found = visitedNodes[rchild]
			if !found {
				hash := rchild.Hash()
				return 0, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
			}
		}

		encNode := flattener.EncodeNode(n, lchildIndex, rchildIndex, scratch)
		_, err := writer.Write(encNode)
		if err != nil {
			return 0, fmt.Errorf("cannot serialize node: %w", err)
		}
	}

	return nodeCounter, nil
}
