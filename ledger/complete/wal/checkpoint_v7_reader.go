package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// OpenAndReadAsPayloadlessTrie reads a checkpoint file (V6 or V7) and returns payloadless tries.
// If the checkpoint is V7, it reads directly as payloadless tries.
// If the checkpoint is V6, it reads the tries and treats them as payloadless
// (since V6 checkpoints created from payloadless forests already store payload hashes as values).
func OpenAndReadAsPayloadlessTrie(dir string, fileName string, logger zerolog.Logger) (
	triesToReturn []*trie.MTrie,
	errToReturn error,
) {
	headerPath := filePathCheckpointHeader(dir, fileName)
	errToReturn = withFile(logger, headerPath, func(file *os.File) error {
		// Read header to determine version
		header := make([]byte, headerSize)
		_, err := io.ReadFull(file, header)
		if err != nil {
			return fmt.Errorf("cannot read header: %w", err)
		}

		magicBytes := binary.BigEndian.Uint16(header)
		version := binary.BigEndian.Uint16(header[encMagicSize:])

		if magicBytes != MagicBytesCheckpointHeader {
			return fmt.Errorf("unknown file format. Magic constant %x does not match expected %x", magicBytes, MagicBytesCheckpointHeader)
		}

		// Reset to start of file
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("cannot seek to start of file: %w", err)
		}

		switch version {
		case VersionV7:
			// V7 is already payloadless, read directly
			tries, err := readCheckpointV7(file, logger)
			if err != nil {
				return err
			}
			triesToReturn = tries
			return nil
		case VersionV6:
			// V6 needs to be read and converted to payloadless
			tries, err := readCheckpointV6AsPayloadless(file, logger)
			if err != nil {
				return err
			}
			triesToReturn = tries
			return nil
		default:
			return fmt.Errorf("unsupported checkpoint version %x for payloadless reading", version)
		}
	})
	return triesToReturn, errToReturn
}

// readCheckpointV6AsPayloadless reads a V6 checkpoint and returns payloadless tries.
// This is useful when the V6 checkpoint was created from a payloadless forest,
// where the payload values are already 32-byte hashes.
func readCheckpointV6AsPayloadless(headerFile *os.File, logger zerolog.Logger) ([]*trie.MTrie, error) {
	headerPath := headerFile.Name()
	dir, fileName := filepath.Split(headerPath)

	lg := logger.With().Str("checkpoint_file", headerPath).Logger()
	lg.Info().Msgf("reading v6 checkpoint file as payloadless")

	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(headerPath, logger)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return nil, fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	// Read subtrie nodes (same as V6, nodes don't change)
	subtrieNodes, err := readSubTriesConcurrently(dir, fileName, subtrieChecksums, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).
		Msg("finish reading all v6 subtrie files, start reading top level tries as payloadless")

	// Read top level tries with payloadless flag
	tries, err := readTopLevelTriesAsPayloadless(dir, fileName, subtrieNodes, topTrieChecksum, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	lg.Info().Msgf("finish reading all trie roots as payloadless, trie root count: %v", len(tries))

	if len(tries) > 0 {
		first, last := tries[0], tries[len(tries)-1]
		logger.Info().
			Str("first_hash", first.RootHash().String()).
			Uint64("first_reg_count", first.AllocatedRegCount()).
			Str("last_hash", last.RootHash().String()).
			Uint64("last_reg_count", last.AllocatedRegCount()).
			Bool("payloadless", true).
			Int("version", 6).
			Msg("checkpoint tries roots (read as payloadless)")
	}

	return tries, nil
}

// readTopLevelTriesAsPayloadless reads top level tries from V6 checkpoint but creates payloadless tries.
func readTopLevelTriesAsPayloadless(dir string, fileName string, subtrieNodes [][]*node.Node, topTrieChecksum uint32, logger zerolog.Logger) (
	rootTriesToReturn []*trie.MTrie,
	errToReturn error,
) {
	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		// read and validate magic bytes and version (V6)
		err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, file)
		if err != nil {
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

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to 0: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read version for top trie: %w", err)
		}

		buf := make([]byte, encNodeCountSize)
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return fmt.Errorf("could not read subtrie node count: %w", err)
		}
		readSubtrieNodeCount, err := decodeNodeCount(buf)
		if err != nil {
			return fmt.Errorf("could not decode node count: %w", err)
		}

		totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)

		if readSubtrieNodeCount != totalSubTrieNodeCount {
			return fmt.Errorf("mismatch subtrie node count, read from disk (%v), but got actual node count (%v)",
				readSubtrieNodeCount, totalSubTrieNodeCount)
		}

		topLevelNodes := make([]*node.Node, topLevelNodesCount+1)
		tries := make([]*trie.MTrie, triesCount)

		scratch := make([]byte, 1024*4)

		for i := uint64(1); i <= topLevelNodesCount; i++ {
			node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				if nodeIndex >= i+uint64(totalSubTrieNodeCount) {
					return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
				}
				return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			})
			if err != nil {
				return fmt.Errorf("cannot read node at index %d: %w", i, err)
			}
			topLevelNodes[i] = node
		}

		// Read trie root nodes with payloadless flag set to true
		for i := uint16(0); i < triesCount; i++ {
			t, err := flattener.ReadTrieWithPayloadless(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			}, true) // isPayloadless = true

			if err != nil {
				return fmt.Errorf("cannot read root trie at index %d: %w", i, err)
			}
			tries[i] = t
		}

		_, err = io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize])
		if err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()

		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
				expectedSum, actualSum)
		}

		_, err = io.ReadFull(reader, scratch[:crc32SumSize])
		if err != nil {
			return fmt.Errorf("could not read checksum from top trie file: %w", err)
		}

		err = ensureReachedEOF(reader)
		if err != nil {
			return fmt.Errorf("fail to read top trie file: %w", err)
		}

		rootTriesToReturn = tries
		return nil
	})
	return rootTriesToReturn, errToReturn
}

// readCheckpointV7 reads checkpoint file from a main file and 17 file parts.
// V7 is identical to V6 in structure but creates payloadless tries.
// The payloadless tries store payload hashes instead of full payloads.
//
// it returns (tries, nil) if there was no error
// it returns (nil, os.ErrNotExist) if a certain file is missing, use (os.IsNotExist to check)
// it returns (nil, ErrEOFNotReached) if a certain part file is malformed
// it returns (nil, err) if running into any exception
func readCheckpointV7(headerFile *os.File, logger zerolog.Logger) ([]*trie.MTrie, error) {
	// the full path of header file
	headerPath := headerFile.Name()
	dir, fileName := filepath.Split(headerPath)

	lg := logger.With().Str("checkpoint_file", headerPath).Logger()
	lg.Info().Msgf("reading v7 payloadless checkpoint file")

	subtrieChecksums, topTrieChecksum, err := readCheckpointHeaderV7(headerPath, logger)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	// ensure all checkpoint part file exists, might return os.ErrNotExist error
	// if a file is missing
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
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

// readCheckpointHeaderV7 reads the V7 checkpoint header file.
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
	// read the magic bytes and check version
	err = validateFileHeader(MagicBytesCheckpointHeader, VersionV7, reader)
	if err != nil {
		return nil, 0, err
	}

	// read the subtrie count
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

	// read top level trie checksum
	topTrieChecksum, err := readCRC32Sum(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read checkpoint top level trie checksum in checkpoint summary: %w", err)
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

	err = ensureReachedEOF(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("fail to read checkpoint header file: %w", err)
	}

	return subtrieChecksums, topTrieChecksum, nil
}

func readSubTriesConcurrentlyV7(dir string, fileName string, subtrieChecksums []uint32, logger zerolog.Logger) ([][]*node.Node, error) {
	numOfSubTries := len(subtrieChecksums)
	jobs := make(chan jobReadSubtrie, numOfSubTries)
	resultChs := make([]<-chan *resultReadSubTrie, numOfSubTries)

	// push all jobs into the channel
	for i, checksum := range subtrieChecksums {
		resultCh := make(chan *resultReadSubTrie)
		resultChs[i] = resultCh
		jobs <- jobReadSubtrie{
			Index:    i,
			Checksum: checksum,
			Result:   resultCh,
		}
	}
	close(jobs)

	nWorker := numOfSubTries
	for i := 0; i < nWorker; i++ {
		go func() {
			for job := range jobs {
				nodes, err := readCheckpointSubTrieV7(dir, fileName, job.Index, job.Checksum, logger)
				job.Result <- &resultReadSubTrie{
					Nodes: nodes,
					Err:   err,
				}
				close(job.Result)
			}
		}()
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

func readCheckpointSubTrieV7(dir string, fileName string, index int, checksum uint32, logger zerolog.Logger) (
	[]*node.Node,
	error,
) {
	var nodes []*node.Node
	err := processCheckpointSubTrieV7(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4)

			nodes = make([]*node.Node, nodesCount+1)
			logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots (v7)", index), int(nodesCount), logger)
			for i := uint64(1); i <= nodesCount; i++ {
				node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
					if nodeIndex >= i {
						return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
					}
					return nodes[nodeIndex], nil
				})
				if err != nil {
					return fmt.Errorf("cannot read node %d: %w", i, err)
				}
				nodes[i] = node
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
		// validate the magic bytes and version (V7 subtrie files use V7 version)
		err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV7, f)
		if err != nil {
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

		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("cannot seek to start of file: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(f, defaultBufioReadSize))

		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read version again for subtrie: %w", err)
		}

		err = processNode(reader, nodesCount)
		if err != nil {
			return err
		}

		scratch := make([]byte, 1024)
		_, err = io.ReadFull(reader, scratch[:encNodeCountSize])
		if err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()

		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in subtrie checkpoint, expected %v, actual %v",
				expectedSum, actualSum)
		}

		_, err = io.ReadFull(reader, scratch[:crc32SumSize])
		if err != nil {
			return fmt.Errorf("could not read subtrie file's checksum: %w", err)
		}

		err = ensureReachedEOF(reader)
		if err != nil {
			return fmt.Errorf("fail to read %v-th subtrie file: %w", index, err)
		}

		return nil
	})
}

// readTopLevelTriesV7 reads the top level tries from V7 checkpoint and creates payloadless tries.
func readTopLevelTriesV7(dir string, fileName string, subtrieNodes [][]*node.Node, topTrieChecksum uint32, logger zerolog.Logger) (
	rootTriesToReturn []*trie.MTrie,
	errToReturn error,
) {
	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		// read and validate magic bytes and version (V7 top trie files use V7 version)
		err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV7, file)
		if err != nil {
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

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to 0: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read version for top trie: %w", err)
		}

		buf := make([]byte, encNodeCountSize)
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return fmt.Errorf("could not read subtrie node count: %w", err)
		}
		readSubtrieNodeCount, err := decodeNodeCount(buf)
		if err != nil {
			return fmt.Errorf("could not decode node count: %w", err)
		}

		totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)

		if readSubtrieNodeCount != totalSubTrieNodeCount {
			return fmt.Errorf("mismatch subtrie node count, read from disk (%v), but got actual node count (%v)",
				readSubtrieNodeCount, totalSubTrieNodeCount)
		}

		topLevelNodes := make([]*node.Node, topLevelNodesCount+1)
		tries := make([]*trie.MTrie, triesCount)

		scratch := make([]byte, 1024*4)

		for i := uint64(1); i <= topLevelNodesCount; i++ {
			node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				if nodeIndex >= i+uint64(totalSubTrieNodeCount) {
					return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
				}
				return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			})
			if err != nil {
				return fmt.Errorf("cannot read node at index %d: %w", i, err)
			}
			topLevelNodes[i] = node
		}

		// Read trie root nodes with payloadless flag set to true
		for i := uint16(0); i < triesCount; i++ {
			t, err := flattener.ReadTrieWithPayloadless(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			}, true) // isPayloadless = true

			if err != nil {
				return fmt.Errorf("cannot read root trie at index %d: %w", i, err)
			}
			tries[i] = t
		}

		_, err = io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize])
		if err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()

		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
				expectedSum, actualSum)
		}

		_, err = io.ReadFull(reader, scratch[:crc32SumSize])
		if err != nil {
			return fmt.Errorf("could not read checksum from top trie file: %w", err)
		}

		err = ensureReachedEOF(reader)
		if err != nil {
			return fmt.Errorf("fail to read top trie file: %w", err)
		}

		rootTriesToReturn = tries
		return nil
	})
	return rootTriesToReturn, errToReturn
}
