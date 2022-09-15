package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

func ReadCheckpointConcurrently(dir string) ([]*trie.MTrie, error) {

	file, err := openTopLevelTrieNodes(dir)
	if err != nil {
		return nil, fmt.Errorf("could not open top leve trie node file: %w", err)
	}
	defer func() {
		// TODO: evict
		_ = file.Close()
	}()

	subtrieNodes, err := readSubTriesConcurrently(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	// validate the subtrie node checksum
	tries, err := readTopLevelTries(file, subtrieNodes)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes count: %w", err)
	}

	return tries, nil
}

func openTopLevelTrieNodes(dir string) (*os.File, error) {
	filepath := path.Join(dir, "17")
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	return file, nil
}

func readNodeCountAndTriesCount(f *os.File) (uint64, uint16, error) {
	// read the node count and tries count from the end of the file
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
	footer := make([]byte, footerSize)                     // must not be less than 1024
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot seek to footer: %w", err)
	}
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, 0, fmt.Errorf("could not read footer: %w", err)
	}

	return decodeFooter(footer)
}

func readSubTriesConcurrently(dir string) ([][]*node.Node, error) {
	resultChs := make([]chan *resultReadSubTrie, 0, 16)
	for i := 0; i < 16; i++ {
		// TODO: move to readCheckpointSubTrie
		f, err := openFileByIndex(dir, i)
		if err != nil {
			return nil, fmt.Errorf("could not open file at index %v: %w", i, err)
		}
		defer func(f *os.File) {
			f.Close()
		}(f)

		resultCh := make(chan *resultReadSubTrie)
		go func(i int) {
			nodes, err := readCheckpointSubTrie(f)
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

func readCheckpointSubTrie(f *os.File) ([]*node.Node, error) {
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

	// TODO: validate checksum

	return nodes, nil
}

type resultReadSubTrie struct {
	Nodes []*node.Node
	Err   error
}

type subTrie struct {
	file      *os.File
	nodeCount uint64
}

func readSubTriesNodeCount(f *os.File) (uint64, error) {
	const footerOffset = encNodeCountSize + crc32SumSize
	const footerSize = encNodeCountSize // footer doesn't include crc32 sum
	footer := make([]byte, footerSize)  // must not be less than 1024
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("cannot seek to footer: %w", err)
	}
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, fmt.Errorf("could not read footer: %w", err)
	}

	return decodeSubtrieFooter(footer)
}

func openFileByIndex(dir string, index int) (*os.File, error) {
	filepath := path.Join(dir, fmt.Sprintf("%v", index))
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	return file, nil
}

// TODO: take subtrie checksum
func readTopLevelTries(file *os.File, subtrieNodes [][]*node.Node) ([]*trie.MTrie, error) {
	// TODO: read header to validate checksums of sub trie nodes
	// TODO: read subtrie count
	// TODO: read subtrie checksum

	topLevelNodesCount, triesCount, err := readNodeCount(file)
	if err != nil {
		return nil, fmt.Errorf("could not read node count: %w", err)
	}

	topLevelNodes := make([]*node.Node, topLevelNodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)
	totalNodeCount := uint64(totalSubTrieNodeCount) + topLevelNodesCount

	reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// read the nodes from subtrie level to the root level
	for i := uint64(totalSubTrieNodeCount) + 1; i <= totalNodeCount; i++ {
		node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex > uint64(i) {
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
			return nil, fmt.Errorf("cannot read trie at index %d: %w", i, err)
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
		total += len(group)
	}
	return total
}

func getNodeByIndex(subtrieNodes [][]*node.Node, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	offset := index
	for _, subtries := range subtrieNodes {
		if index < uint64(len(subtries)) {
			return subtries[offset], nil
		}
		offset -= uint64(len(subtries))
	}

	if offset >= uint64(len(topLevelNodes)) {
		return nil, fmt.Errorf("can not find node by index: %v", index)
	}

	return topLevelNodes[offset], nil
}
