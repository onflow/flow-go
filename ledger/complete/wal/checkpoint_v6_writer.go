package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	utilsio "github.com/onflow/flow-go/utils/io"
)

const subtrieLevel = 4
const subtrieCount = 1 << subtrieLevel // 16

func subtrieCountByLevel(level uint16) int {
	return 1 << level
}

// StoreCheckpointV6SingleThread stores checkpoint file in v6 in a single threaded manner,
// useful when EN is executing block.
func StoreCheckpointV6SingleThread(tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV6(tries, outputDir, outputFile, logger, 1)
}

// StoreCheckpointV6Concurrently stores checkpoint file in v6 in max workers,
// useful during state extraction
func StoreCheckpointV6Concurrently(tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger) error {
	return StoreCheckpointV6(tries, outputDir, outputFile, logger, 16)
}

// StoreCheckpointV6 stores checkpoint file into a main file and 17 file parts.
// the main file stores:
//   - version
//   - checksum of each part file (17 in total)
//   - checksum of the main file itself
//     the first 16 files parts contain the trie nodes below the subtrieLevel
//     the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
//
// nWorker specifies how many workers to encode subtrie concurrently, valid range [1,16]
func StoreCheckpointV6(
	tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint) error {
	err := storeCheckpointV6(tries, outputDir, outputFile, logger, nWorker)
	if err != nil {
		cleanupErr := deleteCheckpointFiles(outputDir, outputFile)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup temp file %s, after running into error: %w", cleanupErr, err)
		}
		return err
	}

	return nil
}

func storeCheckpointV6(
	tries []*trie.MTrie, outputDir string, outputFile string, logger zerolog.Logger, nWorker uint) error {
	if len(tries) == 0 {
		logger.Info().Msg("no tries to be checkpointed")
		return nil
	}

	first, last := tries[0], tries[len(tries)-1]
	lg := logger.With().
		Int("version", 6).
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
		Msg("storing checkpoint")

	// make sure a checkpoint file with same name doesn't exist
	// part file with same name doesn't exist either
	matched, err := findCheckpointPartFiles(outputDir, outputFile)
	if err != nil {
		return fmt.Errorf("fail to check if checkpoint file already exist: %w", err)
	}

	// found checkpoint file with the same checkpoint number
	if len(matched) != 0 {
		return fmt.Errorf("checkpoint part file already exists: %v", matched)
	}

	subtrieRoots := createSubTrieRoots(tries)

	subTrieRootIndices, subTriesNodeCount, subTrieChecksums, err := storeSubTrieConcurrently(
		subtrieRoots,
		estimateSubtrieNodeCount(last), // considering the last trie most likely have more registers than others
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

	topTrieChecksum, err := storeTopLevelNodesAndTrieRoots(
		tries, subTrieRootIndices, subTriesNodeCount, outputDir, outputFile, lg)
	if err != nil {
		return fmt.Errorf("could not store top level tries: %w", err)
	}

	err = storeCheckpointHeader(subTrieChecksums, topTrieChecksum, outputDir, outputFile, lg)
	if err != nil {
		return fmt.Errorf("could not store checkpoint header: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).Msg("checkpoint file has been successfully stored")

	return nil
}

// 1. version
// 2. checksum of each part file (17 in total)
// 3. checksum of the main file itself
func storeCheckpointHeader(
	subTrieChecksums []uint32,
	topTrieChecksum uint32,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	errToReturn error,
) {
	// sanity check
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

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointHeader, VersionV6))
	if err != nil {
		return fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	// encode subtrieCount
	_, err = writer.Write(encodeSubtrieCount(subtrieCount))
	if err != nil {
		return fmt.Errorf("cannot write subtrie level into checkpoint header: %w", err)
	}

	//  write subtrie checksums
	for i, subtrieSum := range subTrieChecksums {
		_, err = writer.Write(encodeCRC32Sum(subtrieSum))
		if err != nil {
			return fmt.Errorf("cannot write %v-th subtriechecksum into checkpoint header: %w", i, err)
		}
	}

	// write top level trie checksum
	_, err = writer.Write(encodeCRC32Sum(topTrieChecksum))
	if err != nil {
		return fmt.Errorf("cannot write top level trie checksum into checkpoint header: %w", err)
	}

	// write checksum to the end of the file
	checksum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(checksum))
	if err != nil {
		return fmt.Errorf("cannot write CRC32 checksum to checkpoint header: %w", err)
	}
	return nil
}

var createWriterForCheckpointHeader = createClosableWriter

// 17th part file contains:
// 1. checkpoint version
// 2. subtrieNodeCount
// 3. top level nodes
// 4. trie roots
// 5. node count
// 6. trie count
// 7. checksum
func storeTopLevelNodesAndTrieRoots(
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
	// the remaining nodes and data will be stored into the same file
	closable, err := createWriterForTopTries(outputDir, outputFile, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(closable, errToReturn)
	}()

	writer := NewCRC32Writer(closable)

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV6))
	if err != nil {
		return 0, fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	// write subTriesNodeCount
	_, err = writer.Write(encodeNodeCount(subTriesNodeCount))
	if err != nil {
		return 0, fmt.Errorf("could not write subtrie node count: %w", err)
	}

	scratch := make([]byte, 1024*4)

	// write top level nodes
	topLevelNodeIndices, topLevelNodesCount, err := storeTopLevelNodes(
		scratch,
		tries,
		subTrieRootIndices,
		subTriesNodeCount+1, // the counter is 1 more than the node count, because the first item is nil
		writer)

	if err != nil {
		return 0, fmt.Errorf("could not store top level nodes: %w", err)
	}

	logger.Info().Msgf("top level nodes have been stored. top level node count: %v", topLevelNodesCount)

	// write tries
	err = storeTries(scratch, tries, topLevelNodeIndices, writer)
	if err != nil {
		return 0, fmt.Errorf("could not store trie root nodes: %w", err)
	}

	// write checksum
	checksum, err := storeTopLevelTrieFooter(topLevelNodesCount, uint16(len(tries)), writer)
	if err != nil {
		return 0, fmt.Errorf("could not store footer: %w", err)
	}

	return checksum, nil
}

func createSubTrieRoots(tries []*trie.MTrie) [subtrieCount][]*node.Node {
	var subtrieRoots [subtrieCount][]*node.Node
	for i := 0; i < len(subtrieRoots); i++ {
		subtrieRoots[i] = make([]*node.Node, len(tries))
	}

	for trieIndex, t := range tries {
		// subtries is an array with subtrieCount trie nodes
		// in breadth-first order at subtrieLevel of the trie `t`
		subtries := getNodesAtLevel(t.RootNode(), subtrieLevel)
		for subtrieIndex, subtrieRoot := range subtries {
			subtrieRoots[subtrieIndex][trieIndex] = subtrieRoot
		}
	}
	return subtrieRoots
}

// estimateSubtrieNodeCount estimate the average number of registers in each subtrie.
func estimateSubtrieNodeCount(trie *trie.MTrie) int {
	estimatedTrieNodeCount := 2*int(trie.AllocatedRegCount()) - 1
	return estimatedTrieNodeCount / subtrieCount
}

// subTrieRootAndTopLevelTrieCount return the total number of subtrie root nodes
// and top level trie nodes for given number of tries
// it is used for preallocating memory for the map that holds all unique nodes in
// all sub trie roots and top level trie nodoes.
// the top level trie nodes has nearly same number of nodes as subtrie node count at subtrieLevel
// that's it needs to * 2.
func subTrieRootAndTopLevelTrieCount(tries []*trie.MTrie) int {
	return len(tries) * subtrieCount * 2
}

type resultStoringSubTrie struct {
	Index     int
	Roots     map[*node.Node]uint64 // node index for root nodes
	NodeCount uint64
	Checksum  uint32
	Err       error
}

type jobStoreSubTrie struct {
	Index  int
	Roots  []*node.Node
	Result chan<- *resultStoringSubTrie
}

func storeSubTrieConcurrently(
	subtrieRoots [subtrieCount][]*node.Node,
	estimatedSubtrieNodeCount int, // useful for preallocating memory for building unique node map when processing sub tries
	subAndTopNodeCount int, // useful for preallocating memory for the node indices map to be returned
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
	nWorker uint,
) (
	map[*node.Node]uint64, // node indices
	uint64, // node count
	[]uint32, //checksums
	error, // any exception
) {
	logger.Info().Msgf("storing %v subtrie groups with average node count %v for each subtrie", subtrieCount, estimatedSubtrieNodeCount)

	if nWorker == 0 || nWorker > subtrieCount {
		return nil, 0, nil, fmt.Errorf("invalid nWorker %v, the valid range is [1,%v]", nWorker, subtrieCount)
	}

	jobs := make(chan jobStoreSubTrie, len(subtrieRoots))
	resultChs := make([]<-chan *resultStoringSubTrie, len(subtrieRoots))

	// push all jobs into the channel
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

	// start nWorker number of goroutine to take the job from the jobs channel concurrently
	// and work on them, after finish, continue until the jobs channel is drained
	for i := 0; i < int(nWorker); i++ {
		go func() {
			for job := range jobs {
				roots, nodeCount, checksum, err := storeCheckpointSubTrie(
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

	// reading job results in the same order as their indices
	for _, resultCh := range resultChs {
		result := <-resultCh

		if result.Err != nil {
			return nil, 0, nil, fmt.Errorf("fail to store %v-th subtrie, trie: %w", result.Index, result.Err)
		}

		for root, index := range result.Roots {
			// nil is always 0
			if root == nil {
				results[root] = 0
			} else {
				// the original index is relative to the subtrie file itself.
				// but we need a global index to be referenced by top level trie,
				// therefore we need to add the nodeCounter
				results[root] = index + nodeCounter
			}
		}
		nodeCounter += result.NodeCount
		checksums = append(checksums, result.Checksum)
	}

	return results, nodeCounter, checksums, nil
}

func createWriterForTopTries(dir string, file string, logger zerolog.Logger) (io.WriteCloser, error) {
	_, topTriesFileName := filePathTopTries(dir, file)

	return createClosableWriter(dir, topTriesFileName, logger)
}

func createWriterForSubtrie(dir string, file string, logger zerolog.Logger, index int) (io.WriteCloser, error) {
	_, subTriesFileName, err := filePathSubTries(dir, file, index)
	if err != nil {
		return nil, err
	}

	return createClosableWriter(dir, subTriesFileName, logger)
}

func createClosableWriter(dir string, fileName string, logger zerolog.Logger) (io.WriteCloser, error) {
	fullPath := path.Join(dir, fileName)
	if utilsio.FileExists(fullPath) {
		return nil, fmt.Errorf("checkpoint part file %v already exists", fullPath)
	}

	tmpFile, err := os.CreateTemp(dir, fmt.Sprintf("writing-%v-*", fileName))
	if err != nil {
		return nil, fmt.Errorf("could not create temporary file for checkpoint toptries: %w", err)
	}

	writer := bufio.NewWriterSize(tmpFile, defaultBufioWriteSize)
	return &SyncOnCloseRenameFile{
		logger:     logger,
		file:       tmpFile,
		targetName: fullPath,
		Writer:     writer,
	}, nil
}

// storeCheckpointSubTrie traverse each root node, and store the subtrie nodes into
// the subtrie part file at index i
// subtrie file contains:
// 1. checkpoint version
// 2. nodes
// 3. node count
// 4. checksum
func storeCheckpointSubTrie(
	i int,
	roots []*node.Node,
	estimatedSubtrieNodeCount int, // for estimate the amount of memory to be preallocated
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
) (
	rootNodesOfAllSubtries map[*node.Node]uint64, // the stored position of each unique root node
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

	// create a CRC32 writer, so that any bytes passed to the writer will
	// be used to calculate CRC32 checksum
	writer := NewCRC32Writer(closable)

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV6))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write version into checkpoint subtrie file: %w", err)
	}

	// subtrieRootNodes unique subtrie root nodes, the uint64 value is the index of each root node
	// stored in the part file.
	subtrieRootNodes := make(map[*node.Node]uint64, len(roots))

	// nodeCounter is counter for all unique nodes.
	// It starts from 1, as 0 marks nil node.
	nodeCounter := uint64(1)

	logging := logProgress(fmt.Sprintf("storing %v-th sub trie roots", i), estimatedSubtrieNodeCount, logger)

	// traversedSubtrieNodes contains all unique nodes of subtries of the same path and their index.
	traversedSubtrieNodes := make(map[*node.Node]uint64, estimatedSubtrieNodeCount)
	// index 0 is nil, it can be used in a node's left child or right child to indicate
	// a node's left child or right child is nil
	traversedSubtrieNodes[nil] = 0

	scratch := make([]byte, 1024*4)
	for _, root := range roots {
		// Note: nodeCounter is to assign an global index to each node in the order of it being seralized
		// into the checkpoint file. Therefore, it has to be reused when iterating each subtrie.
		// storeUniqueNodes will add the unique visited node into traversedSubtrieNodes with key as the node
		// itself, and value as n-th node being seralized in the checkpoint file.
		nodeCounter, err = storeUniqueNodes(root, traversedSubtrieNodes, nodeCounter, scratch, writer, logging)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("fail to store nodes in step 1 for subtrie root %v: %w", root.Hash(), err)
		}
		// Save subtrie root node index in topLevelNodes,
		// so when traversing top level tries
		// (from level 0 to subtrieLevel) using topLevelNodes,
		// node iterator skips subtrie as visited nodes.
		subtrieRootNodes[root] = traversedSubtrieNodes[root]
	}

	// -1 to account for 0 node meaning nil
	totalNodeCount := nodeCounter - 1

	// write total number of node as footer
	checksum, err := storeSubtrieFooter(totalNodeCount, writer)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not store subtrie footer %w", err)
	}

	return subtrieRootNodes, totalNodeCount, checksum, nil
}

func storeTopLevelNodes(
	scratch []byte,
	tries []*trie.MTrie,
	subTrieRootIndices map[*node.Node]uint64,
	initNodeCounter uint64,
	writer io.Writer) (
	map[*node.Node]uint64,
	uint64,
	error) {
	nodeCounter := initNodeCounter
	var err error
	for _, t := range tries {
		root := t.RootNode()
		if root == nil {
			continue
		}
		// if we iterate through the root trie with an empty visited nodes map, then it will iterate through
		// all nodes at all levels. In order to skip the nodes above subtrieLevel, since they have been seralized in step 1,
		// we will need to pass in a visited nodes map that contains all the subtrie root nodes, which is the topLevelNodes.
		// The topLevelNodes was built in step 1, when seralizing each subtrie root nodes.
		nodeCounter, err = storeUniqueNodes(root, subTrieRootIndices, nodeCounter, scratch, writer, func(uint64) {})
		if err != nil {
			return nil, 0, fmt.Errorf("fail to store nodes in step 2 for root trie %v: %w", root.Hash(), err)
		}
	}

	topLevelNodesCount := nodeCounter - initNodeCounter
	return subTrieRootIndices, topLevelNodesCount, nil
}

func storeTries(
	scratch []byte,
	tries []*trie.MTrie,
	topLevelNodes map[*node.Node]uint64,
	writer io.Writer) error {
	for _, t := range tries {
		rootNode := t.RootNode()
		if !t.IsEmpty() && rootNode.Height() != ledger.NodeMaxHeight {
			return fmt.Errorf("height of root node must be %d, but is %d",
				ledger.NodeMaxHeight, rootNode.Height())
		}

		// Get root node index
		rootIndex, found := topLevelNodes[rootNode]
		if !found {
			rootHash := t.RootHash()
			return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(rootHash[:]))
		}

		encTrie := flattener.EncodeTrie(t, rootIndex, scratch)
		_, err := writer.Write(encTrie)
		if err != nil {
			return fmt.Errorf("cannot serialize trie: %w", err)
		}
	}

	return nil
}

// deleteCheckpointFiles removes any checkpoint files with given checkpoint prefix in the outputDir.
func deleteCheckpointFiles(outputDir string, outputFile string) error {
	pattern := filePathPattern(outputDir, outputFile)
	filesToRemove, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("could not glob checkpoint files to delete with pattern %v: %w",
			pattern, err,
		)
	}

	var merror *multierror.Error
	for _, file := range filesToRemove {
		err := os.Remove(file)
		if err != nil {
			merror = multierror.Append(merror, err)
		}
	}

	return merror.ErrorOrNil()
}

func storeTopLevelTrieFooter(topLevelNodesCount uint64, rootTrieCount uint16, writer *Crc32Writer) (uint32, error) {
	footer := encodeTopLevelNodesAndTriesFooter(topLevelNodesCount, rootTrieCount)
	_, err := writer.Write(footer)
	if err != nil {
		return 0, fmt.Errorf("cannot write checkpoint footer: %w", err)
	}

	// write checksum to the end of the file
	checksum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(checksum))
	if err != nil {
		return 0, fmt.Errorf("cannot write CRC32 checksum to top level part file: %w", err)
	}

	return checksum, nil
}

func storeSubtrieFooter(nodeCount uint64, writer *Crc32Writer) (uint32, error) {
	footer := encodeNodeCount(nodeCount)
	_, err := writer.Write(footer)
	if err != nil {
		return 0, fmt.Errorf("cannot write checkpoint subtrie footer: %w", err)
	}

	// write checksum to the end of the file
	crc32Sum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(crc32Sum))
	if err != nil {
		return 0, fmt.Errorf("cannot write CRC32 checksum %v", err)
	}
	return crc32Sum, nil
}

func encodeTopLevelNodesAndTriesFooter(topLevelNodesCount uint64, rootTrieCount uint16) []byte {
	footer := make([]byte, encNodeCountSize+encTrieCountSize)
	binary.BigEndian.PutUint64(footer, topLevelNodesCount)
	binary.BigEndian.PutUint16(footer[encNodeCountSize:], rootTrieCount)
	return footer
}

func decodeTopLevelNodesAndTriesFooter(footer []byte) (uint64, uint16, error) {
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
	if len(footer) != footerSize {
		return 0, 0, fmt.Errorf("wrong footer size, expect %v, got %v", footerSize, len(footer))
	}
	nodesCount := binary.BigEndian.Uint64(footer)
	triesCount := binary.BigEndian.Uint16(footer[encNodeCountSize:])
	return nodesCount, triesCount, nil
}

func encodeNodeCount(nodeCount uint64) []byte {
	buf := make([]byte, encNodeCountSize)
	binary.BigEndian.PutUint64(buf, nodeCount)
	return buf
}

func decodeNodeCount(encoded []byte) (uint64, error) {
	if len(encoded) != encNodeCountSize {
		return 0, fmt.Errorf("wrong subtrie node count size, expect %v, got %v", encNodeCountSize, len(encoded))
	}
	return binary.BigEndian.Uint64(encoded), nil
}

func encodeCRC32Sum(checksum uint32) []byte {
	buf := make([]byte, crc32SumSize)
	binary.BigEndian.PutUint32(buf, checksum)
	return buf
}

func decodeCRC32Sum(encoded []byte) (uint32, error) {
	if len(encoded) != crc32SumSize {
		return 0, fmt.Errorf("wrong crc32sum size, expect %v, got %v", crc32SumSize, len(encoded))
	}
	return binary.BigEndian.Uint32(encoded), nil
}

func encodeVersion(magic uint16, version uint16) []byte {
	// Write header: magic (2 bytes) + version (2 bytes)
	header := make([]byte, encMagicSize+encVersionSize)
	binary.BigEndian.PutUint16(header, magic)
	binary.BigEndian.PutUint16(header[encMagicSize:], version)
	return header
}

func decodeVersion(encoded []byte) (uint16, uint16, error) {
	if len(encoded) != encMagicSize+encVersionSize {
		return 0, 0, fmt.Errorf("wrong version size, expect %v, got %v", encMagicSize+encVersionSize, len(encoded))
	}
	magicBytes := binary.BigEndian.Uint16(encoded)
	version := binary.BigEndian.Uint16(encoded[encMagicSize:])
	return magicBytes, version, nil
}

func encodeSubtrieCount(level uint16) []byte {
	bytes := make([]byte, encSubtrieCountSize)
	binary.BigEndian.PutUint16(bytes, level)
	return bytes
}

func decodeSubtrieCount(encoded []byte) (uint16, error) {
	if len(encoded) != encSubtrieCountSize {
		return 0, fmt.Errorf("wrong subtrie level size, expect %v, got %v", encSubtrieCountSize, len(encoded))
	}
	return binary.BigEndian.Uint16(encoded), nil
}

// closeAndMergeError close the closable and merge the closeErr with the given err into a multierror
// Note: when using this function in a defer function, don't use as below:
// func XXX() (
//
//	err error,
//	) {
//		def func() {
//			// bad, because the definition of err might get overwritten
//			err = closeAndMergeError(closable, err)
//		}()
//
// Better to use as below:
// func XXX() (
//
//	errToReturn error,
//	) {
//		def func() {
//			// good, because the error to returned is only updated here, and guaranteed to be returned
//			errToReturn = closeAndMergeError(closable, errToReturn)
//		}()
func closeAndMergeError(closable io.Closer, err error) error {
	var merr *multierror.Error
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	closeError := closable.Close()
	if closeError != nil {
		merr = multierror.Append(merr, closeError)
	}

	return merr.ErrorOrNil()
}

// withFile opens the file at the given path, and calls the given function with the opened file.
// it handles closing the file and evicting the file from Linux page cache.
func withFile(logger zerolog.Logger, filepath string, f func(file *os.File) error) (
	errToReturn error,
) {

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(file *os.File) {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict top trie file %s from Linux page cache: %s", filepath, evictErr)
			// No need to return this error because it's possible to continue normal operations.
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(file)

	return f(file)
}
