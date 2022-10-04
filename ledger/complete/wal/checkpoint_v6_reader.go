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
)

// ReadCheckpointV6 reads checkpoint file from a main file and 17 file parts.
// the main file stores:
// 		1. version
//		2. checksum of each part file (17 in total)
// 		3. checksum of the main file itself
// 	the first 16 files parts contain the trie nodes below the subtrieLevel
//	the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
// it returns (tries, nil) if there was no error
// it returns (nil, os.ErrNotExist) if a certain file is missing, use (os.IsNotExist to check)
// it returns (nil, err) if running into any exception
func ReadCheckpointV6(dir string, fileName string) ([]*trie.MTrie, error) {
	// TODO: read the main file and check the version
	// TODO: read the checksum of 17 file parts

	headerPath := filePathHeader(dir, fileName)
	subtrieChecksums, topTrieChecksum, err := readHeader(headerPath)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	// ensure all checkpoint part file exists, might return os.ErrNotExist error
	// if a file is missing
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return nil, fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	subtrieNodes, err := readSubTriesConcurrently(dir, fileName, subtrieChecksums)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	// validate the subtrie node checksum
	tries, err := readTopLevelTries(dir, fileName, subtrieNodes, topTrieChecksum)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	return tries, nil
}

func filePathHeader(dir string, fileName string) string {
	return path.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) (string, string, error) {
	if index < 0 || index > 15 {
		return "", "", fmt.Errorf("index must be between 1 to 16, but got %v", index)
	}
	subTrieFileName := fmt.Sprintf("%s.%03d", fileName, index)
	return path.Join(dir, subTrieFileName), subTrieFileName, nil
}

func filePathTopTries(dir string, fileName string) (string, string) {
	topTriesFileName := fmt.Sprintf("%s.%03d", fileName, 16)
	return path.Join(dir, topTriesFileName), fileName
}

// TODO: to add
func readHeader(filePath string) ([]uint32, uint32, error) {
	return make([]uint32, 16), 0, nil
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

// allPartFileExist check if all the part files of the checkpoint file exist
// it returns nil if all files exist
// it returns os.ErrNotExist if some file is missing, use (os.IsNotExist to check)
// it returns err if running into any exception
func allPartFileExist(dir string, fileName string, totalSubtrieFiles int) error {
	// check all subtrie part file exist
	for i := 0; i < totalSubtrieFiles; i++ {
		filePath, _, err := filePathSubTries(dir, fileName, i)
		if err != nil {
			return fmt.Errorf("fail to find file path for %v-th subtrie file: %w", i, err)
		}

		// ensure file exists
		_, err = os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("fail to check %v-th subtrie file exist: %w", i, err)
		}
	}

	// check top level part file exist
	toplevelPartFile, _ := filePathTopTries(dir, fileName)
	_, err := os.Stat(toplevelPartFile)
	if err != nil {
		return fmt.Errorf("fail to check top level file exist: %w", err)
	}

	return nil
}

var errCheckpointFileExist = errors.New("checkpoint file exists already")

// noneCheckpointFileExist check if none of the checkpoint header file or the part files exist
// it returns nil if none exists
// it returns errCheckpointFileExist if a checkpoint file exists already
// it returns err if running into any other exception
func noneCheckpointFileExist(dir string, fileName string, totalSubtrieFiles int) error {
	// ensure header file does not exist
	headerPath := filePathHeader(dir, fileName)
	_, err := os.Stat(headerPath)
	if err == nil {
		return fmt.Errorf("checkpoint header file already exist: %v", headerPath)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("could not check header file exist: %w", err)
	}

	// ensure part file does not exist
	for i := 0; i < totalSubtrieFiles; i++ {
		filePath, _, err := filePathSubTries(dir, fileName, i)
		if err != nil {
			return fmt.Errorf("fail to find file path for %v-th subtrie file: %w", i, err)
		}

		// ensure file exists
		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			continue
		}

		// file exists
		if err == nil {
			return fmt.Errorf("checkpoint %v-th part file: %w", i, errCheckpointFileExist)
		}

		if err != nil {
			return fmt.Errorf("fail to check %v-th part file exist: %w", i, err)
		}
	}
	return nil
}

func readSubTriesConcurrently(dir string, fileName string, subtrieChecksums []uint32) ([][]*node.Node, error) {
	// TODO: replace 16 with const
	if len(subtrieChecksums) != 16 {
		return nil, fmt.Errorf("expect subtrieChecksums to be %v, but got %v", 16, len(subtrieChecksums))
	}

	resultChs := make([]chan *resultReadSubTrie, 0, len(subtrieChecksums))
	for i := 0; i < 16; i++ {
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

	nodesGroups := make([][]*node.Node, 0, len(resultChs))
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
	// TODO: read and validate checksum

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

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	var bufReader io.Reader = bufio.NewReaderSize(f, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)

	scratch := make([]byte, 1024*4)           // must not be less than 1024
	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	for i := uint64(1); i <= nodesCount; i++ {
		node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= i {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = node
	}

	// TODO: validate checksum again

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

	footer := make([]byte, footerSize)
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, fmt.Errorf("could not read footer: %w", err)
	}

	return decodeSubtrieFooter(footer)
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
	// TODO: read header to validate checksums of sub trie nodes
	// TODO: read subtrie count
	// TODO: read subtrie checksum

	filepath, _ := filePathTopTries(dir, fileName)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func() {
		// TODO: evict
		_ = file.Close()
	}()

	// TODO: read checksum and validate

	topLevelNodesCount, triesCount, err := readNodeCount(file)
	if err != nil {
		return nil, fmt.Errorf("could not read node count: %w", err)
	}

	topLevelNodes := make([]*node.Node, topLevelNodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)
	// TODO: read subtrie Node count and validate

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("could not seek to 0: %w", err)
	}

	reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

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

	// TODO: validate checksum

	return tries, nil
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
