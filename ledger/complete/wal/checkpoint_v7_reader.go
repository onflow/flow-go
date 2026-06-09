package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// readCheckpointV7 reads a payloadless checkpoint from a header file and 17 part
// files, returning the reconstructed []*payloadless.MTrie.
//
// It returns:
//   - (tries, nil) on success
//   - (nil, os.ErrNotExist) if a part file is missing (callers can use [os.IsNotExist])
//   - (nil, ErrEOFNotReached) if a part file is malformed at the trailing bytes
//   - (nil, err) for any other exception
func readCheckpointV7(headerFile *os.File, logger zerolog.Logger) ([]*payloadless.MTrie, error) {
	headerPath := headerFile.Name()
	dir, fileName := filepath.Split(headerPath)

	lg := logger.With().Str("checkpoint_file", headerPath).Logger()
	lg.Info().Msgf("reading v7 payloadless checkpoint file")

	subtrieChecksums, topTrieChecksum, err := readCheckpointHeaderV7(headerPath, logger)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	if err := allPartFileExist(dir, fileName, len(subtrieChecksums)); err != nil {
		return nil, fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	subtrieNodes, err := readSubTriesConcurrentlyV7(dir, fileName, subtrieChecksums, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).
		Msg("finish reading all v7 subtrie files, start reading top level tries")

	tries, err := readTopLevelTriesV7(dir, fileName, subtrieNodes, topTrieChecksum, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	lg.Info().Msgf("finish reading all payloadless trie roots, trie root count: %v", len(tries))

	if len(tries) > 0 {
		first, last := tries[0], tries[len(tries)-1]
		logger.Info().
			Str("first_hash", first.RootHash().String()).
			Uint64("first_reg_count", first.AllocatedRegCount()).
			Str("last_hash", last.RootHash().String()).
			Uint64("last_reg_count", last.AllocatedRegCount()).
			Bool("payloadless", true).
			Int("version", 7).
			Msg("checkpoint tries roots")
	}

	return tries, nil
}

// OpenAndReadCheckpointV7 opens a V7 (payloadless) checkpoint and returns the tries
// as []*payloadless.MTrie. The file must be a V7 checkpoint — V6 (and any other
// version) is rejected, both because the V7 reader explicitly validates the V7
// magic+version at every part-file header and because V7 files use a different
// filename suffix ([V7FileSuffix]) so they're trivially distinguishable on disk.
func OpenAndReadCheckpointV7(dir string, fileName string, logger zerolog.Logger) (
	triesToReturn []*payloadless.MTrie,
	errToReturn error,
) {
	headerPath := filePathCheckpointHeader(dir, fileName)
	errToReturn = withFile(logger, headerPath, func(file *os.File) error {
		tries, err := readCheckpointV7(file, logger)
		if err != nil {
			return err
		}
		triesToReturn = tries
		return nil
	})
	return triesToReturn, errToReturn
}

// readCheckpointHeaderV7 reads and validates the V7 checkpoint header file,
// returning the per-subtrie checksums and the top-trie file checksum.
func readCheckpointHeaderV7(filepath string, logger zerolog.Logger) (
	checksumsOfSubtries []uint32,
	checksumOfTopTrie uint32,
	errToReturn error,
) {
	closable, err := os.Open(filepath)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open header file: %w", err)
	}
	defer func(file *os.File) {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict header file %s from Linux page cache: %s", filepath, evictErr)
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(closable)

	var bufReader io.Reader = bufio.NewReaderSize(closable, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)
	if err := validateFileHeader(MagicBytesCheckpointHeader, VersionV7, reader); err != nil {
		return nil, 0, err
	}

	subtrieCount, err := readSubtrieCount(reader)
	if err != nil {
		return nil, 0, err
	}

	subtrieChecksums := make([]uint32, subtrieCount)
	for i := uint16(0); i < subtrieCount; i++ {
		sum, err := readCRC32Sum(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("could not read %v-th subtrie checksum from checkpoint header: %w", i, err)
		}
		subtrieChecksums[i] = sum
	}

	topTrieChecksum, err := readCRC32Sum(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read checkpoint top level trie checksum in checkpoint summary: %w", err)
	}

	actualSum := reader.Crc32()
	expectedSum, err := readCRC32Sum(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read checkpoint header checksum: %w", err)
	}
	if actualSum != expectedSum {
		return nil, 0, fmt.Errorf("invalid checksum in checkpoint header, expected %v, actual %v",
			expectedSum, actualSum)
	}
	if err := ensureReachedEOF(reader); err != nil {
		return nil, 0, fmt.Errorf("fail to read checkpoint header file: %w", err)
	}
	return subtrieChecksums, topTrieChecksum, nil
}

type payloadlessJobReadSubtrie struct {
	Index    int
	Checksum uint32
	Result   chan<- *payloadlessResultReadSubTrie
}

type payloadlessResultReadSubTrie struct {
	Nodes []*payloadless.Node
	Err   error
}

func readSubTriesConcurrentlyV7(dir string, fileName string, subtrieChecksums []uint32, logger zerolog.Logger) ([][]*payloadless.Node, error) {
	numOfSubTries := len(subtrieChecksums)
	jobs := make(chan payloadlessJobReadSubtrie, numOfSubTries)
	resultChs := make([]<-chan *payloadlessResultReadSubTrie, numOfSubTries)

	for i, checksum := range subtrieChecksums {
		resultCh := make(chan *payloadlessResultReadSubTrie)
		resultChs[i] = resultCh
		jobs <- payloadlessJobReadSubtrie{Index: i, Checksum: checksum, Result: resultCh}
	}
	close(jobs)

	nWorker := numOfSubTries
	for i := 0; i < nWorker; i++ {
		go func() {
			for job := range jobs {
				nodes, err := readCheckpointSubTrieV7(dir, fileName, job.Index, job.Checksum, logger)
				job.Result <- &payloadlessResultReadSubTrie{Nodes: nodes, Err: err}
				close(job.Result)
			}
		}()
	}

	nodesGroups := make([][]*payloadless.Node, 0, len(resultChs))
	for i, resultCh := range resultChs {
		result := <-resultCh
		if result.Err != nil {
			return nil, fmt.Errorf("fail to read %v-th subtrie, trie: %w", i, result.Err)
		}
		nodesGroups = append(nodesGroups, result.Nodes)
	}
	return nodesGroups, nil
}

func readCheckpointSubTrieV7(dir string, fileName string, index int, checksum uint32, logger zerolog.Logger) (
	[]*payloadless.Node,
	error,
) {
	var nodes []*payloadless.Node
	err := processCheckpointSubTrieV7(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4)
			nodes = make([]*payloadless.Node, nodesCount+1)
			logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots (v7)", index), int(nodesCount), logger)
			for i := uint64(1); i <= nodesCount; i++ {
				n, err := payloadless.ReadPayloadlessNode(reader, scratch, func(nodeIndex uint64) (*payloadless.Node, error) {
					if nodeIndex >= i {
						return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
					}
					return nodes[nodeIndex], nil
				})
				if err != nil {
					return fmt.Errorf("cannot read node %d: %w", i, err)
				}
				nodes[i] = n
				logging(i)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return nodes[1:], nil
}

func processCheckpointSubTrieV7(
	dir string,
	fileName string,
	index int,
	checksum uint32,
	logger zerolog.Logger,
	processNode func(*Crc32Reader, uint64) error,
) error {
	filepath, _, err := filePathSubTries(dir, fileName, index)
	if err != nil {
		return err
	}
	return withFile(logger, filepath, func(f *os.File) error {
		if err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV7, f); err != nil {
			return err
		}

		nodesCount, expectedSum, err := readSubTriesFooter(f)
		if err != nil {
			return fmt.Errorf("cannot read sub trie node count: %w", err)
		}
		if checksum != expectedSum {
			return fmt.Errorf("mismatch checksum in subtrie file. checksum from checkpoint header %v does not "+
				"match with the checksum in subtrie file %v", checksum, expectedSum)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("cannot seek to start of file: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(f, defaultBufioReadSize))
		if _, _, err := readFileHeader(reader); err != nil {
			return fmt.Errorf("could not read version again for subtrie: %w", err)
		}

		if err := processNode(reader, nodesCount); err != nil {
			return err
		}

		scratch := make([]byte, 1024)
		if _, err := io.ReadFull(reader, scratch[:encNodeCountSize]); err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()
		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in subtrie checkpoint, expected %v, actual %v",
				expectedSum, actualSum)
		}

		if _, err := io.ReadFull(reader, scratch[:crc32SumSize]); err != nil {
			return fmt.Errorf("could not read subtrie file's checksum: %w", err)
		}
		if err := ensureReachedEOF(reader); err != nil {
			return fmt.Errorf("fail to read %v-th subtrie file: %w", index, err)
		}
		return nil
	})
}

// readTopLevelTriesV7 reads the top-level nodes and trie root records from the
// V7 top-trie part file, resolving each node reference against the previously-read
// subtrie nodes and the running top-level node table.
func readTopLevelTriesV7(dir string, fileName string, subtrieNodes [][]*payloadless.Node, topTrieChecksum uint32, logger zerolog.Logger) (
	rootTriesToReturn []*payloadless.MTrie,
	errToReturn error,
) {
	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		if err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV7, file); err != nil {
			return err
		}

		topLevelNodesCount, triesCount, expectedSum, err := readTopTriesFooter(file)
		if err != nil {
			return fmt.Errorf("could not read top tries footer: %w", err)
		}
		if topTrieChecksum != expectedSum {
			return fmt.Errorf("mismatch top trie checksum, header file has %v, toptrie file has %v",
				topTrieChecksum, expectedSum)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("could not seek to 0: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))
		if _, _, err := readFileHeader(reader); err != nil {
			return fmt.Errorf("could not read version for top trie: %w", err)
		}

		buf := make([]byte, encNodeCountSize)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return fmt.Errorf("could not read subtrie node count: %w", err)
		}
		readSubtrieNodeCount, err := decodeNodeCount(buf)
		if err != nil {
			return fmt.Errorf("could not decode node count: %w", err)
		}

		totalSubTrieNodeCount := computeTotalPayloadlessSubTrieNodeCount(subtrieNodes)
		if readSubtrieNodeCount != totalSubTrieNodeCount {
			return fmt.Errorf("mismatch subtrie node count, read from disk (%v), but got actual node count (%v)",
				readSubtrieNodeCount, totalSubTrieNodeCount)
		}

		topLevelNodes := make([]*payloadless.Node, topLevelNodesCount+1)
		tries := make([]*payloadless.MTrie, triesCount)

		scratch := make([]byte, 1024*4)

		for i := uint64(1); i <= topLevelNodesCount; i++ {
			n, err := payloadless.ReadPayloadlessNode(reader, scratch, func(nodeIndex uint64) (*payloadless.Node, error) {
				if nodeIndex >= i+totalSubTrieNodeCount {
					return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
				}
				return getPayloadlessNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			})
			if err != nil {
				return fmt.Errorf("cannot read node at index %d: %w", i, err)
			}
			topLevelNodes[i] = n
		}

		for i := uint16(0); i < triesCount; i++ {
			t, err := payloadless.ReadPayloadlessTrie(reader, scratch, func(nodeIndex uint64) (*payloadless.Node, error) {
				return getPayloadlessNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			})
			if err != nil {
				return fmt.Errorf("cannot read root trie at index %d: %w", i, err)
			}
			tries[i] = t
		}

		if _, err := io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize]); err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()
		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
				expectedSum, actualSum)
		}

		if _, err := io.ReadFull(reader, scratch[:crc32SumSize]); err != nil {
			return fmt.Errorf("could not read checksum from top trie file: %w", err)
		}
		if err := ensureReachedEOF(reader); err != nil {
			return fmt.Errorf("fail to read top trie file: %w", err)
		}

		rootTriesToReturn = tries
		return nil
	})
	return rootTriesToReturn, errToReturn
}

// computeTotalPayloadlessSubTrieNodeCount returns the total node count across
// all subtrie node groups.
func computeTotalPayloadlessSubTrieNodeCount(subtrieNodes [][]*payloadless.Node) uint64 {
	total := 0
	for _, nodes := range subtrieNodes {
		total += len(nodes)
	}
	return uint64(total)
}

// getPayloadlessNodeByIndex resolves a node reference assigned during
// [storeUniquePayloadlessNodes]. Index 0 is the nil sentinel; indices in
// [1, totalSubTrieNodeCount] map into the flattened subtrie node groups; higher
// indices map into topLevelNodes (offset by totalSubTrieNodeCount).
func getPayloadlessNodeByIndex(
	subtrieNodes [][]*payloadless.Node,
	totalSubTrieNodeCount uint64,
	topLevelNodes []*payloadless.Node,
	index uint64,
) (*payloadless.Node, error) {
	if index == 0 {
		return nil, nil
	}
	if index > totalSubTrieNodeCount {
		nodePos := index - totalSubTrieNodeCount
		if nodePos >= uint64(len(topLevelNodes)) {
			return nil, fmt.Errorf("can not find payloadless node by index %v: nodePos %v >= len(topLevelNodes) %v",
				index, nodePos, len(topLevelNodes))
		}
		return topLevelNodes[nodePos], nil
	}
	offset := index - 1
	for _, subtries := range subtrieNodes {
		if int(offset) < len(subtries) {
			return subtries[offset], nil
		}
		offset -= uint64(len(subtries))
	}
	return nil, fmt.Errorf("could not find payloadless node by index %v, totalSubTrieNodeCount %v", index, totalSubTrieNodeCount)
}
