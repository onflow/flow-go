package wal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// ErrEOFNotReached for indicating end of file not reached error
var ErrEOFNotReached = errors.New("expect to reach EOF, but actually didn't")

func ReadTriesRootHash(logger zerolog.Logger, dir string, fileName string) (
	[]ledger.RootHash,
	error,
) {
	err := validateCheckpointFile(logger, dir, fileName)
	if err != nil {
		return nil, err
	}
	return readTriesRootHash(logger, dir, fileName)
}

var CheckpointHasRootHash = checkpointHasRootHash

// readCheckpointV6 reads checkpoint file from a main file and 17 file parts.
// the main file stores:
//   - version
//   - checksum of each part file (17 in total)
//   - checksum of the main file itself
//     the first 16 files parts contain the trie nodes below the subtrieLevel
//     the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
//
// it returns (tries, nil) if there was no error
// it returns (nil, os.ErrNotExist) if a certain file is missing, use (os.IsNotExist to check)
// it returns (nil, ErrEOFNotReached) if a certain part file is malformed
// it returns (nil, err) if running into any exception
func readCheckpointV6(headerFile *os.File, logger zerolog.Logger) ([]*trie.MTrie, error) {
	// the full path of header file
	headerPath := headerFile.Name()
	dir, fileName := filepath.Split(headerPath)

	lg := logger.With().Str("checkpoint_file", headerPath).Logger()
	lg.Info().Msgf("reading v6 checkpoint file")

	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(headerPath, logger)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	// ensure all checkpoint part file exists, might return os.ErrNotExist error
	// if a file is missing
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return nil, fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	// TODO making number of goroutine configable for reading subtries, which can help us
	// test the code on machines that don't have as much RAM as EN by using fewer goroutines.
	subtrieNodes, err := readSubTriesConcurrently(dir, fileName, subtrieChecksums, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).
		Msg("finish reading all v6 subtrie files, start reading top level tries")

	tries, err := readTopLevelTries(dir, fileName, subtrieNodes, topTrieChecksum, lg)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	lg.Info().Msgf("finish reading all trie roots, trie root count: %v", len(tries))

	if len(tries) > 0 {
		first, last := tries[0], tries[len(tries)-1]
		logger.Info().
			Str("first_hash", first.RootHash().String()).
			Uint64("first_reg_count", first.AllocatedRegCount()).
			Str("last_hash", last.RootHash().String()).
			Uint64("last_reg_count", last.AllocatedRegCount()).
			Int("version", 6).
			Msg("checkpoint tries roots")
	}

	return tries, nil
}

// OpenAndReadCheckpointV6 open the checkpoint file and read it with readCheckpointV6
func OpenAndReadCheckpointV6(dir string, fileName string, logger zerolog.Logger) (
	triesToReturn []*trie.MTrie,
	errToReturn error,
) {

	filepath := filePathCheckpointHeader(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		tries, err := readCheckpointV6(file, logger)
		if err != nil {
			return err
		}
		triesToReturn = tries
		return nil
	})

	return triesToReturn, errToReturn
}

// ReadCheckpointFileSize returns the total size of the checkpoint file
func ReadCheckpointFileSize(dir string, fileName string) (uint64, error) {
	paths := allFilePaths(dir, fileName)
	totalSize := uint64(0)
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return 0, fmt.Errorf("could not get file info for %v: %w", path, err)
		}

		totalSize += uint64(fileInfo.Size())
	}

	return totalSize, nil
}

func allFilePaths(dir string, fileName string) []string {
	paths := make([]string, 0, 1+subtrieCount+1)
	paths = append(paths, filePathCheckpointHeader(dir, fileName))
	for i := 0; i < subtrieCount; i++ {
		subTriePath, _, _ := filePathSubTries(dir, fileName, i)
		paths = append(paths, subTriePath)
	}
	topTriePath, _ := filePathTopTries(dir, fileName)
	paths = append(paths, topTriePath)
	return paths
}

func filePathCheckpointHeader(dir string, fileName string) string {
	return path.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) (string, string, error) {
	if index < 0 || index > (subtrieCount-1) {
		return "", "", fmt.Errorf("index must be between 0 to %v, but got %v", subtrieCount-1, index)
	}
	subTrieFileName := partFileName(fileName, index)
	return path.Join(dir, subTrieFileName), subTrieFileName, nil
}

func filePathTopTries(dir string, fileName string) (string, string) {
	topTriesFileName := partFileName(fileName, subtrieCount)
	return path.Join(dir, topTriesFileName), topTriesFileName
}

func partFileName(fileName string, index int) string {
	return fmt.Sprintf("%v.%03d", fileName, index)
}

func filePathPattern(dir string, fileName string) string {
	return fmt.Sprintf("%v*", filePathCheckpointHeader(dir, fileName))
}

// readCheckpointHeader takes a file path and returns subtrieChecksums and topTrieChecksum
// any error returned are exceptions
func readCheckpointHeader(filepath string, logger zerolog.Logger) (
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
			// No need to return this error because it's possible to continue normal operations.
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(closable)

	var bufReader io.Reader = bufio.NewReaderSize(closable, defaultBufioReadSize)
	reader := NewCRC32Reader(bufReader)
	// read the magic bytes and check version
	err = validateFileHeader(MagicBytesCheckpointHeader, VersionV6, reader)
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

	err = ensureReachedEOF(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("fail to read checkpoint header file: %w", err)
	}

	return subtrieChecksums, topTrieChecksum, nil
}

// allPartFileExist check if all the part files of the checkpoint file exist
// it returns nil if all files exist
// it returns os.ErrNotExist if some file is missing, use (os.IsNotExist to check)
// it returns err if running into any exception
func allPartFileExist(dir string, fileName string, totalSubtrieFiles int) error {
	matched, err := findCheckpointPartFiles(dir, fileName)
	if err != nil {
		return fmt.Errorf("could not check all checkpoint part file exist: %w", err)
	}

	// header + subtrie files + top level file
	if len(matched) != 1+totalSubtrieFiles+1 {
		return fmt.Errorf("some checkpoint part file is missing. found part files %v. err :%w",
			matched, os.ErrNotExist)
	}

	return nil
}

// findCheckpointPartFiles returns a slice of file full paths of the part files for the checkpoint file
// with the given fileName under the given folder.
// - it return the matching part files, note it might not contains all the part files.
// - it return error if running any exception
func findCheckpointPartFiles(dir string, fileName string) ([]string, error) {
	pattern := filePathPattern(dir, fileName)
	matched, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("could not find checkpoint files: %w", err)
	}

	// build a lookup with matched
	lookup := make(map[string]struct{})
	for _, match := range matched {
		lookup[match] = struct{}{}
	}

	headerPath := filePathCheckpointHeader(dir, fileName)
	parts := make([]string, 0)
	// check header exists
	_, ok := lookup[headerPath]
	if ok {
		parts = append(parts, headerPath)
		delete(lookup, headerPath)
	}

	// check all subtrie parts
	for i := 0; i < subtrieCount; i++ {
		subtriePath, _, err := filePathSubTries(dir, fileName, i)
		if err != nil {
			return nil, err
		}
		_, ok := lookup[subtriePath]
		if ok {
			parts = append(parts, subtriePath)
			delete(lookup, subtriePath)
		}
	}

	// check top level trie part file
	toplevelPath, _ := filePathTopTries(dir, fileName)

	_, ok = lookup[toplevelPath]
	if ok {
		parts = append(parts, toplevelPath)
		delete(lookup, toplevelPath)
	}

	return parts, nil
}

type jobReadSubtrie struct {
	Index    int
	Checksum uint32
	Result   chan<- *resultReadSubTrie
}

type resultReadSubTrie struct {
	Nodes []*node.Node
	Err   error
}

func readSubTriesConcurrently(dir string, fileName string, subtrieChecksums []uint32, logger zerolog.Logger) ([][]*node.Node, error) {

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

	// TODO: make nWorker configable
	nWorker := numOfSubTries // use as many worker as the jobs to read subtries concurrently
	for i := 0; i < nWorker; i++ {
		go func() {
			for job := range jobs {
				nodes, err := readCheckpointSubTrie(dir, fileName, job.Index, job.Checksum, logger)
				job.Result <- &resultReadSubTrie{
					Nodes: nodes,
					Err:   err,
				}
				close(job.Result)
			}
		}()
	}

	// reading job results in the same order as their indices
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

func readCheckpointSubTrie(dir string, fileName string, index int, checksum uint32, logger zerolog.Logger) (
	[]*node.Node,
	error,
) {
	var nodes []*node.Node
	err := processCheckpointSubTrie(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4) // must not be less than 1024

			nodes = make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
			logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots", index), int(nodesCount), logger)
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

	// since nodes[0] is always `nil`, returning a slice without nodes[0] could simplify the
	// implementation of getNodeByIndex
	// return nodes[1:], nil
	return nodes[1:], nil
}

// subtrie file contains:
// 1. checkpoint version
// 2. nodes
// 3. node count
// 4. checksum
func processCheckpointSubTrie(
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
		// valite the magic bytes and version
		err := validateFileHeader(MagicBytesCheckpointSubtrie, VersionV6, f)
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

		// restart from the beginning of the file, make sure Crc32Reader has seen all the bytes
		// in order to compute the correct checksum
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("cannot seek to start of file: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(f, defaultBufioReadSize))

		// read version again for calculating checksum
		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read version again for subtrie: %w", err)
		}

		// read file part index and verify

		err = processNode(reader, nodesCount)
		if err != nil {
			return err
		}

		scratch := make([]byte, 1024)
		// read footer and discard, since we only care about checksum
		_, err = io.ReadFull(reader, scratch[:encNodeCountSize])
		if err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		// calculate the actual checksum
		actualSum := reader.Crc32()

		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in subtrie checkpoint, expected %v, actual %v",
				expectedSum, actualSum)
		}

		// read the checksum and discard, since we only care about whether ensureReachedEOF
		_, err = io.ReadFull(reader, scratch[:crc32SumSize])
		if err != nil {
			return fmt.Errorf("could not read subtrie file's checksum: %w", err)
		}

		err = ensureReachedEOF(reader)
		if err != nil {
			return fmt.Errorf("fail to read %v-th sutrie file: %w", index, err)
		}

		return nil
	})
}

func readSubTriesFooter(f *os.File) (uint64, uint32, error) {
	const footerSize = encNodeCountSize // footer doesn't include crc32 sum
	const footerOffset = footerSize + crc32SumSize
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot seek to footer: %w", err)
	}

	footer := make([]byte, footerSize)
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, 0, fmt.Errorf("could not read footer: %w", err)
	}

	nodeCount, err := decodeNodeCount(footer)
	if err != nil {
		return 0, 0, fmt.Errorf("could not decode subtrie node count: %w", err)
	}

	// the subtrie checksum from the checkpoint header file must be same
	// as the checksum included in the subtrie file
	expectedSum, err := readCRC32Sum(f)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read checksum for sub trie file: %w", err)
	}

	return nodeCount, expectedSum, nil
}

// 17th part file contains:
// 1. checkpoint version
// 2. subtrieNodeCount
// 3. top level nodes
// 4. trie roots
// 5. node count
// 6. trie count
// 7. checksum
func readTopLevelTries(dir string, fileName string, subtrieNodes [][]*node.Node, topTrieChecksum uint32, logger zerolog.Logger) (
	rootTriesToReturn []*trie.MTrie,
	errToReturn error,
) {

	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		// read and validate magic bytes and version
		err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, file)
		if err != nil {
			return err
		}

		// read subtrie Node count and validate
		topLevelNodesCount, triesCount, expectedSum, err := readTopTriesFooter(file)
		if err != nil {
			return fmt.Errorf("could not read top tries footer: %w", err)
		}

		if topTrieChecksum != expectedSum {
			return fmt.Errorf("mismatch top trie checksum, header file has %v, toptrie file has %v",
				topTrieChecksum, expectedSum)
		}

		// restart from the beginning of the file, make sure CRC32Reader has seen all the bytes
		// in order to compute the correct checksum
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to 0: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

		// read version again for calculating checksum
		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read version for top trie: %w", err)
		}

		// read subtrie count and validate
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

		topLevelNodes := make([]*node.Node, topLevelNodesCount+1) //+1 for 0 index meaning nil
		tries := make([]*trie.MTrie, triesCount)

		// Scratch buffer is used as temporary buffer that reader can read into.
		// Raw data in scratch buffer should be copied or converted into desired
		// objects before next Read operation.  If the scratch buffer isn't large
		// enough, a new buffer will be allocated.  However, 4096 bytes will
		// be large enough to handle almost all payloads and 100% of interim nodes.
		scratch := make([]byte, 1024*4) // must not be less than 1024

		// read the nodes from subtrie level to the root level
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

		// read the trie root nodes
		for i := uint16(0); i < triesCount; i++ {
			trie, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
			})

			if err != nil {
				return fmt.Errorf("cannot read root trie at index %d: %w", i, err)
			}
			tries[i] = trie
		}

		// read footer and discard, since we only care about checksum
		_, err = io.ReadFull(reader, scratch[:encNodeCountSize+encTrieCountSize])
		if err != nil {
			return fmt.Errorf("cannot read footer: %w", err)
		}

		actualSum := reader.Crc32()

		if actualSum != expectedSum {
			return fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
				expectedSum, actualSum)
		}

		// read the checksum and discard, since we only care about whether ensureReachedEOF
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

func readTriesRootHash(logger zerolog.Logger, dir string, fileName string) (
	trieRootsToReturn []ledger.RootHash,
	errToReturn error,
) {

	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		var err error

		// read and validate magic bytes and version
		err = validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, file)
		if err != nil {
			return err
		}

		// read subtrie Node count and validate
		_, triesCount, _, err := readTopTriesFooter(file)
		if err != nil {
			return fmt.Errorf("could not read top tries footer: %w", err)
		}

		footerOffset := encNodeCountSize + encTrieCountSize + crc32SumSize
		trieRootOffset := footerOffset + flattener.EncodedTrieSize*int(triesCount)

		_, err = file.Seek(int64(-trieRootOffset), io.SeekEnd)
		if err != nil {
			return fmt.Errorf("could not seek to 0: %w", err)
		}

		reader := bufio.NewReaderSize(file, defaultBufioReadSize)
		trieRoots := make([]ledger.RootHash, 0, triesCount)
		scratch := make([]byte, 1024*4) // must not be less than 1024
		for i := 0; i < int(triesCount); i++ {
			trieRootNode, err := flattener.ReadEncodedTrie(reader, scratch)
			if err != nil {
				return fmt.Errorf("could not read trie root node: %w", err)
			}

			trieRoots = append(trieRoots, ledger.RootHash(trieRootNode.RootHash))
		}

		trieRootsToReturn = trieRoots
		return nil
	})
	return trieRootsToReturn, errToReturn
}

// checkpointHasRootHash check if the given checkpoint file contains the expected root hash
func checkpointHasRootHash(logger zerolog.Logger, bootstrapDir, filename string, expectedRootHash ledger.RootHash) error {
	roots, err := ReadTriesRootHash(logger, bootstrapDir, filename)
	if err != nil {
		return fmt.Errorf("could not read checkpoint root hash: %w", err)
	}

	if len(roots) == 0 {
		return fmt.Errorf("no root hash found in checkpoint file")
	}

	for i, root := range roots {
		if root == expectedRootHash {
			logger.Info().Msgf("found matching checkpoint root hash at index: %v, checkpoint total trie roots: %v",
				i, len(roots))
			// found the expected commit
			return nil
		}
	}

	return fmt.Errorf("could not find expected root hash %v in checkpoint file which contains: %v ", expectedRootHash, roots)
}

func readFileHeader(reader io.Reader) (uint16, uint16, error) {
	bytes := make([]byte, encMagicSize+encVersionSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read magic ID and version: %w", err)
	}
	return decodeVersion(bytes)
}

func validateFileHeader(expectedMagic uint16, expectedVersion uint16, reader io.Reader) error {
	magic, version, err := readFileHeader(reader)
	if err != nil {
		return err
	}

	if magic != expectedMagic {
		return fmt.Errorf("wrong magic bytes, expect %#x, bot got: %#x", expectedMagic, magic)
	}

	if version != expectedVersion {
		return fmt.Errorf("wrong version, expect %v, bot got: %v", expectedVersion, version)
	}

	return nil
}

func readSubtrieCount(reader io.Reader) (uint16, error) {
	bytes := make([]byte, encSubtrieCountSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, err
	}
	return decodeSubtrieCount(bytes)
}

func readCRC32Sum(reader io.Reader) (uint32, error) {
	bytes := make([]byte, crc32SumSize)
	_, err := io.ReadFull(reader, bytes)
	if err != nil {
		return 0, err
	}
	return decodeCRC32Sum(bytes)
}

func readTopTriesFooter(f *os.File) (uint64, uint16, uint32, error) {
	// footer offset: nodes count (8 bytes) + tries count (2 bytes) + CRC32 sum (4 bytes)
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum
	// Seek to footer
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("cannot seek to footer: %w", err)
	}
	footer := make([]byte, footerSize)
	_, err = io.ReadFull(f, footer)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("cannot read footer: %w", err)
	}

	nodeCount, trieCount, err := decodeTopLevelNodesAndTriesFooter(footer)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("could not decode top trie footer: %w", err)
	}

	checksum, err := readCRC32Sum(f)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("cannot read checksum for top trie file: %w", err)
	}
	return nodeCount, trieCount, checksum, nil
}

func computeTotalSubTrieNodeCount(groups [][]*node.Node) uint64 {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	return uint64(total)
}

// get a node by node index.
// Note: node index start from 1.
// subtries contains subtrie node groups. subtries[i][0] is NOT nil.
// topLevelNodes contains top level nodes. topLevelNodes[0] is nil.
// any error returned are exceptions
func getNodeByIndex(subtrieNodes [][]*node.Node, totalSubTrieNodeCount uint64, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	if index == 0 {
		// item at index 0 is for nil
		return nil, nil
	}

	if index > totalSubTrieNodeCount {
		return getTopNodeByIndex(totalSubTrieNodeCount, topLevelNodes, index)
	}

	offset := index - 1 // index > 0, won't underflow
	for _, subtries := range subtrieNodes {
		if int(offset) < len(subtries) {
			return subtries[offset], nil
		}

		offset -= uint64(len(subtries))
	}

	return nil, fmt.Errorf("could not find node by index %v, totalSubTrieNodeCount %v", index, totalSubTrieNodeCount)
}

func getTopNodeByIndex(totalSubTrieNodeCount uint64, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	nodePos := index - totalSubTrieNodeCount

	if nodePos >= uint64(len(topLevelNodes)) {
		return nil, fmt.Errorf("can not find node by index %v, nodePos >= len(topLevelNodes) => (%v > %v)",
			index, nodePos, len(topLevelNodes))
	}

	return topLevelNodes[nodePos], nil
}

// ensureReachedEOF checks if the reader has reached end of file
// it returns nil if reached EOF
// it returns ErrEOFNotReached if didn't reach end of file
// any error returned are exception
func ensureReachedEOF(reader io.Reader) error {
	b := make([]byte, 1)
	_, err := reader.Read(b)
	if errors.Is(err, io.EOF) {
		return nil
	}

	if err == nil {
		return ErrEOFNotReached
	}

	return fmt.Errorf("fail to check if reached EOF: %w", err)
}

func validateCheckpointFile(logger zerolog.Logger, dir, fileName string) error {
	headerPath := filePathCheckpointHeader(dir, fileName)
	// validate header file
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(headerPath, logger)
	if err != nil {
		return err
	}

	// validate subtrie files
	for index, expectedSum := range subtrieChecksums {
		filepath, _, err := filePathSubTries(dir, fileName, index)
		if err != nil {
			return err
		}
		err = withFile(logger, filepath, func(f *os.File) error {
			_, checksum, err := readSubTriesFooter(f)
			if err != nil {
				return fmt.Errorf("cannot read sub trie node count: %w", err)
			}

			if checksum != expectedSum {
				return fmt.Errorf("mismatch checksum in subtrie file. checksum from checkpoint header %v does not "+
					"match with the checksum in subtrie file %v", checksum, expectedSum)
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	// validate top trie file
	filepath, _ := filePathTopTries(dir, fileName)
	err = withFile(logger, filepath, func(file *os.File) error {
		// read subtrie Node count and validate
		_, _, checkSum, err := readTopTriesFooter(file)
		if err != nil {
			return err
		}

		if topTrieChecksum != checkSum {
			return fmt.Errorf("mismatch top trie checksum, header file has %v, toptrie file has %v",
				topTrieChecksum, checkSum)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
