package wal

import (
	"encoding/hex"
	"fmt"
	"io"
	"path"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// StoreCheckpointV7SingleThread stores a V7 (payloadless) checkpoint in a
// single-threaded manner.
func StoreCheckpointV7SingleThread(tries []*payloadless.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV7(tries, outputDir, outputFile, logger, 1)
}

// StoreCheckpointV7Concurrently stores a V7 (payloadless) checkpoint using up to
// 16 worker goroutines to encode subtries in parallel.
func StoreCheckpointV7Concurrently(tries []*payloadless.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV7(tries, outputDir, outputFile, logger, 16)
}

// StoreCheckpointV7 stores a payloadless checkpoint into a header file and 17 part
// files. The on-disk layout (header + 16 subtrie parts + top-trie part) mirrors V6,
// but each node and trie record is encoded by the payloadless flattener
// ([payloadless.EncodeNode], [payloadless.EncodeTrie]) — leaves carry a 32-byte
// leaf hash, not a full payload.
//
// nWorker specifies how many subtries to encode concurrently; valid range is [1,16].
func StoreCheckpointV7(
	tries []*payloadless.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint,
) error {
	if err := storeCheckpointV7(tries, outputDir, outputFile, logger, nWorker); err != nil {
		cleanupErr := deleteCheckpointFiles(outputDir, outputFile)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup temp file %s, after running into error: %w", cleanupErr, err)
		}
		return err
	}
	return nil
}

func storeCheckpointV7(
	tries []*payloadless.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint,
) error {
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
		Str("last_hash", last.RootHash().String()).
		Uint64("last_reg_count", last.AllocatedRegCount()).
		Msg("storing payloadless checkpoint")

	// Refuse to clobber any existing part files for this checkpoint name.
	matched, err := findCheckpointPartFiles(outputDir, outputFile)
	if err != nil {
		return fmt.Errorf("fail to check if checkpoint file already exist: %w", err)
	}
	if len(matched) != 0 {
		return fmt.Errorf("checkpoint part file already exists: %v", matched)
	}

	subtrieRoots := createPayloadlessSubTrieRoots(tries)

	subTrieRootIndices, subTriesNodeCount, subTrieChecksums, err := storeSubTrieConcurrentlyV7(
		subtrieRoots,
		estimatePayloadlessSubtrieNodeCount(last),
		payloadlessSubTrieRootAndTopLevelTrieCount(tries),
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

	if err := storeCheckpointHeaderV7(subTrieChecksums, topTrieChecksum, outputDir, outputFile, lg); err != nil {
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
) (errToReturn error) {
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

	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointHeader, VersionV7)); err != nil {
		return fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}
	if _, err := writer.Write(encodeSubtrieCount(subtrieCount)); err != nil {
		return fmt.Errorf("cannot write subtrie level into checkpoint header: %w", err)
	}
	for i, subtrieSum := range subTrieChecksums {
		if _, err := writer.Write(encodeCRC32Sum(subtrieSum)); err != nil {
			return fmt.Errorf("cannot write %v-th subtriechecksum into checkpoint header: %w", i, err)
		}
	}
	if _, err := writer.Write(encodeCRC32Sum(topTrieChecksum)); err != nil {
		return fmt.Errorf("cannot write top level trie checksum into checkpoint header: %w", err)
	}
	if _, err := writer.Write(encodeCRC32Sum(writer.Crc32())); err != nil {
		return fmt.Errorf("cannot write CRC32 checksum to checkpoint header: %w", err)
	}
	return nil
}

func storeTopLevelNodesAndTrieRootsV7(
	tries []*payloadless.MTrie,
	subTrieRootIndices map[*payloadless.Node]uint64,
	subTriesNodeCount uint64,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (checksumOfTopTriePartFile uint32, errToReturn error) {
	closable, err := createWriterForTopTries(outputDir, outputFile, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)

	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV7)); err != nil {
		return 0, fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}
	if _, err := writer.Write(encodeNodeCount(subTriesNodeCount)); err != nil {
		return 0, fmt.Errorf("could not write subtrie node count: %w", err)
	}

	scratch := make([]byte, 1024*4)

	topLevelNodeIndices, topLevelNodesCount, err := storeTopLevelPayloadlessNodes(
		scratch,
		tries,
		subTrieRootIndices,
		subTriesNodeCount+1,
		writer,
	)
	if err != nil {
		return 0, fmt.Errorf("could not store top level nodes: %w", err)
	}

	logger.Info().Msgf("top level nodes have been stored. top level node count: %v", topLevelNodesCount)

	if err := storePayloadlessTries(scratch, tries, topLevelNodeIndices, writer); err != nil {
		return 0, fmt.Errorf("could not store trie root nodes: %w", err)
	}

	checksum, err := storeTopLevelTrieFooter(topLevelNodesCount, uint16(len(tries)), writer)
	if err != nil {
		return 0, fmt.Errorf("could not store footer: %w", err)
	}
	return checksum, nil
}

type payloadlessJobStoreSubTrie struct {
	Index  int
	Roots  []*payloadless.Node
	Result chan<- *payloadlessResultStoringSubTrie
}

type payloadlessResultStoringSubTrie struct {
	Index     int
	Roots     map[*payloadless.Node]uint64
	NodeCount uint64
	Checksum  uint32
	Err       error
}

func storeSubTrieConcurrentlyV7(
	subtrieRoots [subtrieCount][]*payloadless.Node,
	estimatedSubtrieNodeCount int,
	subAndTopNodeCount int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
	nWorker uint,
) (map[*payloadless.Node]uint64, uint64, []uint32, error) {
	logger.Info().Msgf("storing %v subtrie groups (v7) with average node count %v for each subtrie", subtrieCount, estimatedSubtrieNodeCount)

	if nWorker == 0 || nWorker > subtrieCount {
		return nil, 0, nil, fmt.Errorf("invalid nWorker %v, the valid range is [1,%v]", nWorker, subtrieCount)
	}

	jobs := make(chan payloadlessJobStoreSubTrie, len(subtrieRoots))
	resultChs := make([]<-chan *payloadlessResultStoringSubTrie, len(subtrieRoots))

	for i, roots := range subtrieRoots {
		resultCh := make(chan *payloadlessResultStoringSubTrie)
		resultChs[i] = resultCh
		jobs <- payloadlessJobStoreSubTrie{Index: i, Roots: roots, Result: resultCh}
	}
	close(jobs)

	for i := 0; i < int(nWorker); i++ {
		go func() {
			for job := range jobs {
				roots, nodeCount, checksum, err := storeCheckpointSubTrieV7(
					job.Index, job.Roots, estimatedSubtrieNodeCount, outputDir, outputFile, logger)
				job.Result <- &payloadlessResultStoringSubTrie{
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

	results := make(map[*payloadless.Node]uint64, subAndTopNodeCount)
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
	roots []*payloadless.Node,
	estimatedSubtrieNodeCount int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	rootNodesOfAllSubtries map[*payloadless.Node]uint64,
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
	if _, err := writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV7)); err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write version into checkpoint subtrie file: %w", err)
	}

	subtrieRootNodes := make(map[*payloadless.Node]uint64, len(roots))
	nodeCounter := uint64(1)

	logging := logProgress(fmt.Sprintf("storing %v-th sub trie roots (v7)", i), estimatedSubtrieNodeCount, logger)

	traversedSubtrieNodes := make(map[*payloadless.Node]uint64, estimatedSubtrieNodeCount)
	traversedSubtrieNodes[nil] = 0

	scratch := make([]byte, 1024*4)
	for _, root := range roots {
		nodeCounter, err = storeUniquePayloadlessNodes(root, traversedSubtrieNodes, nodeCounter, scratch, writer, logging)
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

// createPayloadlessSubTrieRoots returns the subtrie root nodes — at depth
// [subtrieLevel] from each trie's root — laid out in breadth-first order. The
// outer index is the subtrie position (0..subtrieCount-1); the inner index is
// the trie position.
func createPayloadlessSubTrieRoots(tries []*payloadless.MTrie) [subtrieCount][]*payloadless.Node {
	var subtrieRoots [subtrieCount][]*payloadless.Node
	for i := 0; i < len(subtrieRoots); i++ {
		subtrieRoots[i] = make([]*payloadless.Node, len(tries))
	}
	for trieIndex, t := range tries {
		subtries := getPayloadlessNodesAtLevel(t.RootNode(), subtrieLevel)
		for subtrieIndex, subtrieRoot := range subtries {
			subtrieRoots[subtrieIndex][trieIndex] = subtrieRoot
		}
	}
	return subtrieRoots
}

// estimatePayloadlessSubtrieNodeCount estimates the average number of nodes in a
// subtrie at [subtrieLevel] for a single payloadless trie, using the same
// 2*regCount-1 heuristic as the full-mtrie variant.
func estimatePayloadlessSubtrieNodeCount(t *payloadless.MTrie) int {
	estimatedTrieNodeCount := 2*int(t.AllocatedRegCount()) - 1
	return estimatedTrieNodeCount / subtrieCount
}

// payloadlessSubTrieRootAndTopLevelTrieCount returns an upper-bound estimate of
// the number of unique subtrie-root and top-level-trie nodes across the given
// tries. Used for preallocation only.
func payloadlessSubTrieRootAndTopLevelTrieCount(tries []*payloadless.MTrie) int {
	return len(tries) * subtrieCount * 2
}

// getPayloadlessNodesAtLevel returns the 2^level nodes at depth `level` of a
// payloadless trie in breadth-first order. Positions with no node are nil; the
// returned slice always has length 2^level.
func getPayloadlessNodesAtLevel(root *payloadless.Node, level uint) []*payloadless.Node {
	nodes := []*payloadless.Node{root}
	nodesLevel := uint(0)
	for nodesLevel < level {
		nextLevel := nodesLevel + 1
		nodesAtNextLevel := make([]*payloadless.Node, 1<<nextLevel)
		for i, n := range nodes {
			if n != nil {
				nodesAtNextLevel[i*2] = n.LeftChild()
				nodesAtNextLevel[i*2+1] = n.RightChild()
			}
		}
		nodes = nodesAtNextLevel
		nodesLevel = nextLevel
	}
	return nodes
}

// storeUniquePayloadlessNodes traverses the trie rooted at `root` in
// Descendents-First order, emits each previously-unvisited node to writer via
// [payloadless.EncodeNode], and assigns it a monotonically-increasing index in
// `visitedNodes`. nodeCounter is the next index to assign on entry; the updated
// counter is returned.
func storeUniquePayloadlessNodes(
	root *payloadless.Node,
	visitedNodes map[*payloadless.Node]uint64,
	nodeCounter uint64,
	scratch []byte,
	writer io.Writer,
	nodeCounterUpdated func(nodeCounter uint64),
) (uint64, error) {
	for itr := payloadless.NewUniqueNodeIterator(root, visitedNodes); itr.Next(); {
		n := itr.Value()
		visitedNodes[n] = nodeCounter
		nodeCounter++
		nodeCounterUpdated(nodeCounter)

		var lchildIndex, rchildIndex uint64
		if lchild := n.LeftChild(); lchild != nil {
			idx, found := visitedNodes[lchild]
			if !found {
				h := lchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(h[:]))
			}
			lchildIndex = idx
		}
		if rchild := n.RightChild(); rchild != nil {
			idx, found := visitedNodes[rchild]
			if !found {
				h := rchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(h[:]))
			}
			rchildIndex = idx
		}

		encNode := payloadless.EncodeNode(n, lchildIndex, rchildIndex, scratch)
		if _, err := writer.Write(encNode); err != nil {
			return 0, fmt.Errorf("cannot serialize payloadless node: %w", err)
		}
	}
	return nodeCounter, nil
}

// storeTopLevelPayloadlessNodes serializes each trie's nodes above
// [subtrieLevel], reusing `subTrieRootIndices` as the seeded visitedNodes map so
// subtrie roots (already written in the subtrie pass) are not re-emitted.
func storeTopLevelPayloadlessNodes(
	scratch []byte,
	tries []*payloadless.MTrie,
	subTrieRootIndices map[*payloadless.Node]uint64,
	initNodeCounter uint64,
	writer io.Writer,
) (map[*payloadless.Node]uint64, uint64, error) {
	nodeCounter := initNodeCounter
	for _, t := range tries {
		root := t.RootNode()
		if root == nil {
			continue
		}
		var err error
		nodeCounter, err = storeUniquePayloadlessNodes(root, subTrieRootIndices, nodeCounter, scratch, writer, func(uint64) {})
		if err != nil {
			return nil, 0, fmt.Errorf("fail to store payloadless nodes in step 2 for root trie %v: %w", root.Hash(), err)
		}
	}
	topLevelNodesCount := nodeCounter - initNodeCounter
	return subTrieRootIndices, topLevelNodesCount, nil
}

// storePayloadlessTries writes each trie's metadata record (root index, reg
// count, root hash). Empty tries use root index 0, which encodes the "nil"
// sentinel expected by [payloadless.ReadPayloadlessTrie].
func storePayloadlessTries(
	scratch []byte,
	tries []*payloadless.MTrie,
	topLevelNodes map[*payloadless.Node]uint64,
	writer io.Writer,
) error {
	for _, t := range tries {
		rootNode := t.RootNode()
		var rootIndex uint64
		if rootNode != nil {
			idx, found := topLevelNodes[rootNode]
			if !found {
				rootHash := t.RootHash()
				return fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(rootHash[:]))
			}
			rootIndex = idx
		}
		encTrie := payloadless.EncodeTrie(t, rootIndex, scratch)
		if _, err := writer.Write(encTrie); err != nil {
			return fmt.Errorf("cannot serialize payloadless trie: %w", err)
		}
	}
	return nil
}
