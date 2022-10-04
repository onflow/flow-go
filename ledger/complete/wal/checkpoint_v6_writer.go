package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	utilsio "github.com/onflow/flow-go/utils/io"
)

const subtrieLevel = 4
const subtrieCount = 1 << subtrieLevel

func subtrieCountByLevel(level uint16) int {
	return 1 << subtrieLevel
}

type NodeEncoder func(node *trie.MTrie, index uint64, scratch []byte) []byte

type resultStoringSubTrie struct {
	Index     int
	Roots     map[*node.Node]uint64 // node index for root nodes
	NodeCount uint64
	Checksum  uint32
	Err       error
}

// StoreCheckpointV6 stores checkpoint file into a main file and 17 file parts.
// the main file stores:
// 		1. version
//		2. checksum of each part file (17 in total)
// 		3. checksum of the main file itself
// 	the first 16 files parts contain the trie nodes below the subtrieLevel
//	the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
func StoreCheckpointV6(
	tries []*trie.MTrie, outputDir string, outputFile string, logger *zerolog.Logger) error {
	if len(tries) == 0 {
		logger.Info().Msg("no tries to be checkpointed")
		return nil
	}

	first, last := tries[0], tries[len(tries)-1]
	logger.Info().
		Str("first_hash", first.RootHash().String()).
		Uint64("first_reg_count", first.AllocatedRegCount()).
		Str("last", last.RootHash().String()).
		Uint64("last_reg_count", last.AllocatedRegCount()).
		Msgf("storing checkpoint for %v tries to %v", len(tries), outputDir)

	subtrieRoots := createSubTrieRoots(tries)

	estimatedSubtrieNodeCount := estimateSubtrieNodeCount(tries)

	subTrieRootIndices, subTriesNodeCount, subTrieChecksums, err := storeSubTrieConcurrently(
		subtrieRoots,
		estimatedSubtrieNodeCount,
		outputDir,
		outputFile,
		logger,
	)
	if err != nil {
		return fmt.Errorf("could not store sub trie: %w", err)
	}

	logger.Info().Msgf("subtrie have been stored. sub trie node count: %v", subTriesNodeCount)

	topTrieChecksum, err := storeTopLevelNodesAndTrieRoots(
		tries, subTrieRootIndices, subTriesNodeCount, outputDir, outputFile, logger)
	if err != nil {
		return fmt.Errorf("could not store top level tries: %w", err)
	}

	err = storeCheckpointHeader(subTrieChecksums, topTrieChecksum, outputDir, outputFile, logger)
	if err != nil {
		return fmt.Errorf("could not store checkpoint header: %w", err)
	}

	logger.Info().Msgf("checkpoint file has been successfully stored at %v/%v", outputDir, outputFile)

	return nil
}

// 		1. version
//		2. subtrieLevel
//		2. checksum of each part file (17 in total)
// 		3. checksum of the main file itself
func storeCheckpointHeader(
	subTrieChecksums []uint32,
	topTrieChecksum uint32,
	outputDir string,
	outputFile string,
	logger *zerolog.Logger,
) error {
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
		closeErr := closable.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	writer := NewCRC32Writer(closable)

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointHeader, VersionV6))
	if err != nil {
		return fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	// encode subtrieLevel
	_, err = writer.Write(encodeSubtrieLevel(subtrieLevel))
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

func createWriterForCheckpointHeader(outputDir string, outputFile string, logger *zerolog.Logger) (io.WriteCloser, error) {
	fullPath := filePathCheckpointHeader(outputDir, outputFile)
	if utilsio.FileExists(fullPath) {
		return nil, fmt.Errorf("checkpoint file already exists at %v", fullPath)
	}

	return createClosableWriter(outputDir, logger, outputFile, fullPath)
}

// 17th part file contains:
// 1. checkpoint version TODO
// 2. checkpoint file part index TODO
// 3. subtrieNodeCount TODO
// 4. top level nodes
// 5. trie roots
// 6. node count
// 7. trie count
// 6. checksum
func storeTopLevelNodesAndTrieRoots(
	tries []*trie.MTrie,
	subTrieRootIndices map[*node.Node]uint64,
	subTriesNodeCount uint64,
	outputDir string,
	outputFile string,
	logger *zerolog.Logger,
) (uint32, error) {
	// the remaining nodes and data will be stored into the same file
	closable, err := createWriterForTopTries(outputDir, outputFile, logger)
	if err != nil {
		return 0, fmt.Errorf("could not create writer for top tries: %w", err)
	}
	defer func() {
		closeErr := closable.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	writer := NewCRC32Writer(closable)

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointToptrie, VersionV6))
	if err != nil {
		return 0, fmt.Errorf("cannot write version into checkpoint header: %w", err)
	}

	topLevelNodeIndices, topLevelNodesCount, err := storeTopLevelNodes(
		tries,
		subTrieRootIndices,
		subTriesNodeCount+1, // the counter is 1 more than the node count, because the first item is nil
		writer)

	if err != nil {
		return 0, fmt.Errorf("could not store top level nodes: %w", err)
	}

	logger.Info().Msgf("top level nodes have been stored. top level node count: %v", topLevelNodesCount)

	err = storeRootNodes(tries, topLevelNodeIndices, flattener.EncodeTrie, writer)
	if err != nil {
		return 0, fmt.Errorf("could not store top level nodes: %w", err)
	}

	err = storeFooter(topLevelNodesCount, uint16(len(tries)), writer)
	if err != nil {
		return 0, fmt.Errorf("could not store footer: %w", err)
	}

	// write checksum to the end of the file
	checksum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(checksum))
	if err != nil {
		return 0, fmt.Errorf("cannot write CRC32 checksum to top level part file: %w", err)
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

// estimateSubtrieNodeCount takes a list of tries, and estimate the average number of registers
// in each subtrie.
func estimateSubtrieNodeCount(tries []*trie.MTrie) int {
	if len(tries) == 0 {
		return 0
	}
	// take the last trie and use the allocatedRegCount from there considering its
	// most likely have more registers than the trie with 0 index.
	estimatedTrieNodeCount := 2*int(tries[len(tries)-1].AllocatedRegCount()) - 1
	return estimatedTrieNodeCount / subtrieCount
}

func storeSubTrieConcurrently(
	subtrieRoots [subtrieCount][]*node.Node,
	estimatedSubtrieNodeCount int,
	outputDir string,
	outputFile string,
	logger *zerolog.Logger,
) (
	map[*node.Node]uint64, // node indices
	uint64, // node count
	[]uint32, //checksums
	error, // any exception
) {
	logger.Info().Msgf("storing %v subtrie groups with average node count %v for each subtrie", subtrieCount, estimatedSubtrieNodeCount)

	resultChs := make([]chan *resultStoringSubTrie, 0, len(subtrieRoots))
	for i, subTrieRoot := range subtrieRoots {
		resultCh := make(chan *resultStoringSubTrie)
		go func(i int, subTrieRoot []*node.Node) {
			roots, nodeCount, checksum, err := storeCheckpointSubTrie(
				i, subTrieRoot, estimatedSubtrieNodeCount, outputDir, outputFile, logger)
			resultCh <- &resultStoringSubTrie{
				Index:     i,
				Roots:     roots,
				NodeCount: nodeCount,
				Checksum:  checksum,
				Err:       err,
			}
			close(resultCh)
		}(i, subTrieRoot)
		resultChs = append(resultChs, resultCh)
	}

	logger.Info().Msgf("subtrie roots have been stored")

	results := make(map[*node.Node]uint64, 1<<(subtrieLevel+1))
	results[nil] = 0
	nodeCounter := uint64(0)
	checksums := make([]uint32, 0, len(results))
	for _, resultCh := range resultChs {
		result := <-resultCh
		if result.Err != nil {
			return nil, 0, nil, fmt.Errorf("fail to store %v-th subtrie, trie: %w", result.Index, result.Err)
		}

		for root, index := range result.Roots {
			// the original index is relative to the subtrie file itself.
			// but we need a global index to be referenced by top level trie,
			// therefore we need to add the nodeCounter
			results[root] = index + nodeCounter
		}
		nodeCounter += result.NodeCount
		checksums = append(checksums, result.Checksum)
	}

	return results, nodeCounter, checksums, nil
}

func createWriterForTopTries(dir string, file string, logger *zerolog.Logger) (io.WriteCloser, error) {
	fullPath, topTriesFileName := filePathTopTries(dir, file)
	if utilsio.FileExists(fullPath) {
		return nil, fmt.Errorf("checkpoint file for top tries %s already exists", fullPath)
	}

	return createClosableWriter(dir, logger, topTriesFileName, fullPath)
}

func createWriterForSubtrie(dir string, file string, logger *zerolog.Logger, index int) (io.WriteCloser, error) {
	fullPath, subTriesFileName, err := filePathSubTries(dir, file, index)
	if err != nil {
		return nil, err
	}
	if utilsio.FileExists(fullPath) {
		return nil, fmt.Errorf("checkpoint file for %v-th sub trie %s already exists", index, fullPath)
	}

	return createClosableWriter(dir, logger, subTriesFileName, fullPath)
}

func createClosableWriter(dir string, logger *zerolog.Logger, fileName string, fullPath string) (io.WriteCloser, error) {
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

// subtrie file contains:
// 1. checkpoint version
// 2. nodes
// 3. node count
// 4. checksum
func storeCheckpointSubTrie(
	i int,
	roots []*node.Node,
	estimatedSubtrieNodeCount int,
	outputDir string,
	outputFile string,
	logger *zerolog.Logger,
) (
	map[*node.Node]uint64, uint64, uint32, error) {

	// traversedSubtrieNodes contains all unique nodes of subtries of the same path and their index.
	traversedSubtrieNodes := make(map[*node.Node]uint64, estimatedSubtrieNodeCount)
	// index 0 is nil, it can be used in a node's left child or right child to indicate
	// a node's left child or right child is nil
	traversedSubtrieNodes[nil] = 0

	closable, err := createWriterForSubtrie(outputDir, outputFile, logger, i)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not create writer for sub trie: %w", err)
	}

	defer func() {
		closeErr := closable.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	// create a CRC32 writer, so that any bytes passed to the writer will
	// be used to calculate CRC32 checksum
	writer := NewCRC32Writer(closable)

	// write version
	_, err = writer.Write(encodeVersion(MagicBytesCheckpointSubtrie, VersionV6))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write version into checkpoint subtrie file: %w", err)
	}

	// topLevelNodes contains all unique nodes of given tries
	// from root to subtrie root and their index
	// (ordered by node traversal sequence).
	// Index 0 is a special case with nil node.
	subtrieRootNodes := make(map[*node.Node]uint64, 1<<(subtrieLevel+1))
	subtrieRootNodes[nil] = 0

	// nodeCounter is counter for all unique nodes.
	// It starts from 1, as 0 marks nil node.
	nodeCounter := uint64(1)

	logging := logProgress(fmt.Sprintf("storing %v-th sub trie roots", i), estimatedSubtrieNodeCount, logger)
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
	footer := encodeSubtrieFooter(totalNodeCount)
	_, err = writer.Write(footer)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write checkpoint subtrie footer: %w", err)
	}

	// write checksum to the end of the file
	crc32Sum := writer.Crc32()
	_, err = writer.Write(encodeCRC32Sum(crc32Sum))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("cannot write CRC32 checksum %v", err)
	}

	return subtrieRootNodes, totalNodeCount, crc32Sum, nil
}

func storeTopLevelNodes(
	tries []*trie.MTrie,
	subTrieRootIndices map[*node.Node]uint64,
	initNodeCounter uint64,
	writer io.Writer) (
	map[*node.Node]uint64,
	uint64,
	error) {
	scratch := make([]byte, 1024*4)
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

func storeRootNodes(
	tries []*trie.MTrie,
	topLevelNodes map[*node.Node]uint64,
	encodeNode NodeEncoder,
	writer io.Writer) error {
	scratch := make([]byte, 1024*4)
	for _, t := range tries {
		rootNode := t.RootNode()

		// Get root node index
		rootIndex, found := topLevelNodes[rootNode]
		if !found {
			rootHash := t.RootHash()
			return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(rootHash[:]))
		}

		encTrie := encodeNode(t, rootIndex, scratch)
		_, err := writer.Write(encTrie)
		if err != nil {
			return fmt.Errorf("cannot serialize trie: %w", err)
		}
	}

	return nil
}

func storeFooter(topLevelNodesCount uint64, rootTrieCount uint16, writer io.Writer) error {
	footer := encodeFooter(topLevelNodesCount, rootTrieCount)
	_, err := writer.Write(footer)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint footer: %w", err)
	}
	return nil
}

func encodeFooter(topLevelNodesCount uint64, rootTrieCount uint16) []byte {
	footer := make([]byte, encNodeCountSize+encTrieCountSize)
	binary.BigEndian.PutUint64(footer, topLevelNodesCount)
	binary.BigEndian.PutUint16(footer[encNodeCountSize:], rootTrieCount)
	return footer
}

func decodeFooter(footer []byte) (uint64, uint16, error) {
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
	if len(footer) != footerSize {
		return 0, 0, fmt.Errorf("wrong footer size, expect %v, got %v", footerSize, len(footer))
	}
	nodesCount := binary.BigEndian.Uint64(footer)
	triesCount := binary.BigEndian.Uint16(footer[encNodeCountSize:])
	return nodesCount, triesCount, nil
}

func encodeSubtrieFooter(totalNodeCount uint64) []byte {
	footer := make([]byte, encNodeCountSize)
	binary.BigEndian.PutUint64(footer, totalNodeCount)
	return footer
}

func decodeSubtrieFooter(footer []byte) (uint64, error) {
	if len(footer) != encNodeCountSize {
		return 0, fmt.Errorf("wrong subtrie footer size, expect %v, got %v", encNodeCountSize, len(footer))
	}
	nodesCount := binary.BigEndian.Uint64(footer)
	return nodesCount, nil
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

func encodeSubtrieLevel(level uint16) []byte {
	bytes := make([]byte, encSubtrieLevelSize)
	binary.BigEndian.PutUint16(bytes, level)
	return bytes
}

func decodeSubtrieLevel(encoded []byte) (uint16, error) {
	if len(encoded) != encSubtrieLevelSize {
		return 0, fmt.Errorf("wrong subtrie level size, expect %v, got %v", encSubtrieLevelSize, len(encoded))
	}
	return binary.BigEndian.Uint16(encoded), nil
}
