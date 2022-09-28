package wal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/rs/zerolog"
)

// ReadCheckpointV6 reads checkpoint file from a main file and 17 file parts.
// the main file stores:
// 		1. version
//		2. checksum of each part file (17 in total)
// 		3. checksum of the main file itself
// 	the first 16 files parts contain the trie nodes below the subtrieLevel
//	the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
func ReadCheckpointV6(dir string, fileName string, logger *zerolog.Logger) ([]*trie.MTrie, error) {
	// TODO: read the main file and check the version

	logger.Info().Msgf("reading v6 checkpoint file")

	headerPath := filePathCheckpointHeader(dir, fileName)
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(headerPath)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	subtrieNodes, err := readSubTriesConcurrently(dir, fileName, subtrieChecksums)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	tries, err := readTopLevelTries(dir, fileName, subtrieNodes, topTrieChecksum)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	return tries, nil
}

func filePathCheckpointHeader(dir string, fileName string) string {
	return path.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) (string, string, error) {
	if index < 0 || index > (subtrieCount-1) {
		return "", "", fmt.Errorf("index must be between 1 to 16, but got %v", index)
	}
	subTrieFileName := fmt.Sprintf("%v.%v", fileName, index)
	return path.Join(dir, subTrieFileName), subTrieFileName, nil
}

func filePathTopTries(dir string, fileName string) (string, string) {
	topTriesFileName := fmt.Sprintf("%v.%v", fileName, subtrieCount)
	return path.Join(dir, topTriesFileName), fileName
}

func filePaths(dir string, fileName string, subtrieLevel uint16) []string {
	paths := make([]string, 0)

	paths = append(paths, filePathCheckpointHeader(dir, fileName))

	subtrieCount := subtrieCountByLevel(subtrieLevel)
	for i := 0; i < subtrieCount; i++ {
		paths = append(paths, path.Join(dir, fmt.Sprintf("%v.%v", fileName, i)))
	}

	p, _ := filePathTopTries(dir, fileName)
	paths = append(paths, p)
	return paths
}

// readCheckpointHeader takes a file path and returns subtrieChecksums and topTrieChecksum
// any error returned are exceptions
func readCheckpointHeader(filepath string) ([]uint32, uint32, error) {
	closable, err := os.Open(filepath)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(f *os.File) {
		f.Close()
	}(closable)

	var bufReader io.Reader = bufio.NewReaderSize(closable, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)

	// read the magic bytes and header
	version, err := readVersion(reader)
	if err != nil {
		return nil, 0, err
	}

	if version != VersionV6 {
		return nil, 0, fmt.Errorf("wrong version: %v", version)
	}

	// read the subtrie level
	subtrieLevel, err := readSubtrieLevel(reader)
	if err != nil {
		return nil, 0, err
	}

	// read subtrie checksums
	subtrieCount := subtrieCountByLevel(subtrieLevel)
	subtrieChecksums := make([]uint32, subtrieCount)
	for i := 0; i < subtrieCount; i++ {
		sum, err := readCRC32Sum(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("could not read %v-th subtrie checksum from checkpoint header: %w", i, err)
		}
		subtrieChecksums[i] = sum
	}

	// read top level trie checksum
	topTrieChecksum, err := readCRC32Sum(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read checkpoint top level trie checksum in chechpoint summary: %w", err)
	}

	// calculate the actual checksum
	actualSum := reader.Crc32()

	// read the stored checksum, and compare with the actual sum
	expectedSum, err := readCRC32Sum(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read checkpoint header checksum: %w", err)
	}

	if actualSum != expectedSum {
		return nil, 0, fmt.Errorf("invalid checksum in checkpoint header, expected %v, actual %v",
			expectedSum, actualSum)
	}

	err = reachedEOF(reader)
	if err != nil {
		return nil, 0, err
	}

	return subtrieChecksums, topTrieChecksum, nil
}

// func readNodeCountAndTriesCount(f *os.File) (uint64, uint16, error) {
// 	// read the node count and tries count from the end of the file
// 	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
// 	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
// 	footer := make([]byte, footerSize)                     // must not be less than 1024
// 	_, err := f.Seek(-footerOffset, io.SeekEnd)
// 	if err != nil {
// 		return 0, 0, fmt.Errorf("cannot seek to footer: %w", err)
// 	}
// 	_, err = io.ReadFull(f, footer)
// 	if err != nil {
// 		return 0, 0, fmt.Errorf("could not read footer: %w", err)
// 	}
//
// 	return decodeFooter(footer)
// }

func readSubTriesConcurrently(dir string, fileName string, subtrieChecksums []uint32) ([][]*node.Node, error) {
	if len(subtrieChecksums) != subtrieCount {
		return nil, fmt.Errorf("expect subtrieChecksums to be %v, but got %v", subtrieCount, len(subtrieChecksums))
	}

	resultChs := make([]chan *resultReadSubTrie, 0, subtrieCount)
	for i := 0; i < subtrieCount; i++ {
		resultCh := make(chan *resultReadSubTrie)
		go func(i int) {
			nodes, err := readCheckpointSubTrie(dir, fileName, i, subtrieChecksums[i])
			resultCh <- &resultReadSubTrie{
				Nodes: nodes,
				Err:   err,
			}
			close(resultCh)
		}(i)
		resultChs = append(resultChs, resultCh)
	}

	nodesGroups := make([][]*node.Node, 0)
	for i, resultCh := range resultChs {
		result := <-resultCh
		if result.Err != nil {
			return nil, fmt.Errorf("fail to read %v-th subtrie, trie: %w", i, result.Err)
		}

		nodesGroups = append(nodesGroups, result.Nodes)
	}

	return nodesGroups, nil
}

// subtrie file contains:
// 1. checkpoint version // TODO
// 2. nodes
// 3. node count
// 4. checksum
func readCheckpointSubTrie(dir string, fileName string, index int, checksum uint32) ([]*node.Node, error) {
	filepath, _, err := filePathSubTries(dir, fileName, index)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(f *os.File) {
		f.Close()
	}(f)

	nodesCount, err := readSubTriesNodeCount(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read sub trie node count: %w", err)
	}

	// the subtrie checksum from the checkpoint header file must be same
	// as the checksum included in the subtrie file
	expectedSum, err := readCRC32Sum(bufio.NewReaderSize(f, defaultBufioReadSize))
	if err != nil {
		return nil, fmt.Errorf("cannot read checksum for sub trie file: %w", err)
	}

	if checksum != expectedSum {
		return nil, fmt.Errorf("mismatch checksum in subtrie file. checksum from checkpoint header %v does not"+
			"match with checksum in subtrie file %v", checksum, expectedSum)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	var bufReader io.Reader = bufio.NewReaderSize(f, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)

	// read file part index and verify
	scratch := make([]byte, 1024*4) // must not be less than 1024
	_, err = io.ReadFull(reader, scratch[:encFilePartIndexSize])
	if err != nil {
		return nil, fmt.Errorf("could not read file part index: %w", err)
	}

	readIndex, err := decodeFileIndex(scratch[:encFilePartIndexSize])
	if err != nil {
		return nil, fmt.Errorf("could not decode file part index: %w", err)
	}

	if index != int(readIndex) {
		return nil, fmt.Errorf("checkpoint file index (%v) mismatch, index in file name does not match with index in file (%v)", index, readIndex)
	}

	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	for i := uint64(1); i <= nodesCount; i++ {
		node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(i) {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = node
	}

	// read footer and discard, since we only care about checksum
	_, err = io.ReadFull(reader, scratch[:encNodeCountSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	// calculate the actual checksum
	actualSum := reader.Crc32()

	// read the stored checksum, and compare with the actual sum
	_, err = readCRC32Sum(reader)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie checkpoint checksum: %w", err)
	}

	if actualSum != expectedSum {
		return nil, fmt.Errorf("invalid checksum in subtrie checkpoint, expected %v, actual %v",
			expectedSum, actualSum)
	}

	// compare the checksum with the checksum stored in checkpoint header file
	if checksum != actualSum {
		return nil, fmt.Errorf("invalid checksum in checkpoint header and subtrie header, expected %v, actual %v",
			checksum, actualSum)
	}

	err = reachedEOF(reader)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

type resultReadSubTrie struct {
	Nodes []*node.Node
	Err   error
}

func readSubTriesNodeCount(f *os.File) (uint64, error) {
	const footerSize = encNodeCountSize // footer doesn't include crc32 sum
	const footerOffset = footerSize + crc32SumSize
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("cannot seek to footer: %w", err)
	}

	footer := make([]byte, footerSize) // must not be less than 1024
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, fmt.Errorf("could not read footer: %w", err)
	}

	return decodeNodeCount(footer)
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
func readTopLevelTries(dir string, fileName string, subtrieNodes [][]*node.Node, topTrieChecksum uint32) ([]*trie.MTrie, error) {
	filepath, _ := filePathTopTries(dir, fileName)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func() {
		// TODO: evict
		_ = file.Close()
	}()

	topLevelNodesCount, triesCount, err := readNodeCount(file)
	if err != nil {
		return nil, fmt.Errorf("could not read node count: %w", err)
	}

	topLevelNodes := make([]*node.Node, topLevelNodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	// the subtrie checksum from the checkpoint header file must be same
	// as the checksum included in the subtrie file
	expectedSum, err := readCRC32Sum(bufio.NewReaderSize(file, defaultBufioReadSize))
	if err != nil {
		return nil, fmt.Errorf("cannot read checksum for sub trie file: %w", err)
	}

	if topTrieChecksum != expectedSum {
		return nil, fmt.Errorf("mismatch checksum in top trie file. checksum from checkpoint header %v does not"+
			"match with checksum in top trie file %v", topTrieChecksum, expectedSum)
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("could not seek to 0: %w", err)
	}

	var bufReader io.Reader = bufio.NewReaderSize(file, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)

	totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)
	// read subtrie count
	buf := make([]byte, encNodeCountSize)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie node count: %w", err)
	}
	readSubtrieNodeCount, err := decodeNodeCount(buf)
	if err != nil {
		return nil, fmt.Errorf("could not decode node count: %w", err)
	}

	if int(readSubtrieNodeCount) != totalSubTrieNodeCount {
		return nil, fmt.Errorf("mismatch subtrie node count, read from disk (%v), but got actual node count (%v)",
			readSubtrieNodeCount, totalSubTrieNodeCount)
	}

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// read the nodes from subtrie level to the root level
	for i := uint64(1); i <= topLevelNodesCount; i++ {
		node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex > i+uint64(totalSubTrieNodeCount) {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}

			return getNodeByIndex(subtrieNodes, topLevelNodes, nodeIndex)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node at index %d: %w", i, err)
		}

		topLevelNodes[i] = node
	}

	// read the trie root nodes
	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			return getNodeByIndex(subtrieNodes, topLevelNodes, nodeIndex)
		})

		if err != nil {
			return nil, fmt.Errorf("cannot read root trie at index %d: %w", i, err)
		}
		tries[i] = trie
	}

	// read footer and discard, since we only care about checksum
	_, err = io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	actualSum := reader.Crc32()

	// read the stored checksum, and compare with the actual sum
	_, err = readCRC32Sum(reader)
	if err != nil {
		return nil, fmt.Errorf("could not read top level trie checksum: %w", err)
	}

	if actualSum != expectedSum {
		return nil, fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
			expectedSum, actualSum)
	}

	err = reachedEOF(reader)
	if err != nil {
		return nil, err
	}

	return tries, nil
}

func readVersion(reader io.Reader) (uint16, error) {
	bytes := make([]byte, encMagicSize+encVersionSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, fmt.Errorf("cannot read version: %w", err)
	}
	magic, version, err := decodeVersion(bytes)
	if err != nil {
		return 0, err
	}
	if magic != MagicBytes {
		return 0, fmt.Errorf("wrong magic bytes %v", magic)
	}

	return version, nil
}

func readSubtrieLevel(reader io.Reader) (uint16, error) {
	bytes := make([]byte, encSubtrieLevelSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, err
	}
	return decodeSubtrieLevel(bytes)

}

func readCRC32Sum(reader io.Reader) (uint32, error) {
	bytes := make([]byte, crc32SumSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, err
	}
	return decodeCRC32Sum(bytes)
}

func readNodeCount(f *os.File) (uint64, uint16, error) {
	// footer offset: nodes count (8 bytes) + tries count (2 bytes) + CRC32 sum (4 bytes)
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
	// Seek to footer
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot seek to footer: %w", err)
	}
	footer := make([]byte, footerSize)
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read footer: %w", err)
	}

	return decodeFooter(footer)
}

func computeTotalSubTrieNodeCount(groups [][]*node.Node) int {
	total := 0
	for _, group := range groups {
		total += (len(group) - 1) // the first item in group is <nil>
	}
	return total
}

// each group in subtrieNodes and topLevelNodes start with `nil`
func getNodeByIndex(subtrieNodes [][]*node.Node, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	if index == 0 {
		// item at index 0 is nil
		return nil, nil
	}
	offset := index - 1 // index.> 0, won't underflow
	for _, subtries := range subtrieNodes {
		if len(subtries) < 1 {
			return nil, fmt.Errorf("subtries should have at least 1 item")
		}
		if subtries[0] != nil {
			return nil, fmt.Errorf("subtrie[0] %v isn't nil", subtries[0])
		}

		if offset < uint64(len(subtries)-1) {
			// +1 because first item is always nil
			return subtries[offset+1], nil
		}
		offset -= uint64(len(subtries) - 1)
	}

	// TODO: move this check outside
	if len(topLevelNodes) < 1 {
		return nil, fmt.Errorf("top trie should have at least 1 item")
	}

	if topLevelNodes[0] != nil {
		return nil, fmt.Errorf("top trie [0] %v isn't nil", topLevelNodes[0])
	}

	if offset >= uint64(len(topLevelNodes)-1) {
		return nil, fmt.Errorf("can not find node by index: %v in subtrieNodes %v, topLevelNodes %v", index, subtrieNodes, topLevelNodes)
	}

	// +1 because first item is always nil
	return topLevelNodes[offset+1], nil
}

// reachedEOF checks if the reader has reached end of file
// it returns nil if reached EOF
// any error returned are exception
func reachedEOF(reader io.Reader) error {
	_, err := reader.Read(make([]byte, 1))
	if errors.Is(err, io.EOF) {
		return nil
	}
	return fmt.Errorf("expect to reach EOF, but didn't: %w", err)
}
