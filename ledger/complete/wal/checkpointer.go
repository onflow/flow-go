package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/metrics"
	utilsio "github.com/onflow/flow-go/utils/io"
)

const checkpointFilenamePrefix = "checkpoint."

const MagicBytes uint16 = 0x2137
const VersionV1 uint16 = 0x01

// Versions was reset while changing trie format, so now bump it to 3 to avoid conflicts
// Version 3 contains a file checksum for detecting corrupted checkpoint files.
const VersionV3 uint16 = 0x03

// Version 4 contains a footer with node count and trie count (previously in the header).
// Version 4 also reduces checkpoint data size.  See EncodeNode() and EncodeTrie() for more details.
const VersionV4 uint16 = 0x04

type Checkpointer struct {
	dir            string
	wal            *DiskWAL
	keyByteSize    int
	forestCapacity int
}

func NewCheckpointer(wal *DiskWAL, keyByteSize int, forestCapacity int) *Checkpointer {
	return &Checkpointer{
		dir:            wal.wal.Dir(),
		wal:            wal,
		keyByteSize:    keyByteSize,
		forestCapacity: forestCapacity,
	}
}

// listCheckpoints returns all the numbers (unsorted) of the checkpoint files, and the number of the last checkpoint.
func (c *Checkpointer) listCheckpoints() ([]int, int, error) {

	list := make([]int, 0)

	files, err := ioutil.ReadDir(c.dir)
	if err != nil {
		return nil, -1, fmt.Errorf("cannot list directory [%s] content: %w", c.dir, err)
	}
	last := -1
	for _, fn := range files {
		if !strings.HasPrefix(fn.Name(), checkpointFilenamePrefix) {
			continue
		}
		justNumber := fn.Name()[len(checkpointFilenamePrefix):]
		k, err := strconv.Atoi(justNumber)
		if err != nil {
			continue
		}

		list = append(list, k)

		// the last check point is the one with the highest number
		if k > last {
			last = k
		}
	}

	return list, last, nil
}

// Checkpoints returns all the checkpoint numbers in asc order
func (c *Checkpointer) Checkpoints() ([]int, error) {
	list, _, err := c.listCheckpoints()
	if err != nil {
		return nil, fmt.Errorf("could not fetch all checkpoints: %w", err)
	}

	sort.Ints(list)

	return list, nil
}

// LatestCheckpoint returns number of latest checkpoint or -1 if there are no checkpoints
func (c *Checkpointer) LatestCheckpoint() (int, error) {
	_, last, err := c.listCheckpoints()
	return last, err
}

// NotCheckpointedSegments - returns numbers of segments which are not checkpointed yet,
// or -1, -1 if there are no segments
func (c *Checkpointer) NotCheckpointedSegments() (from, to int, err error) {

	latestCheckpoint, err := c.LatestCheckpoint()
	if err != nil {
		return -1, -1, fmt.Errorf("cannot get last checkpoint: %w", err)
	}

	first, last, err := c.wal.Segments()
	if err != nil {
		return -1, -1, fmt.Errorf("cannot get range of segments: %w", err)
	}

	// there are no segments at all, there is nothing to checkpoint
	if first == -1 && last == -1 {
		return -1, -1, nil
	}

	// no checkpoints
	if latestCheckpoint == -1 {
		return first, last, nil
	}

	// segments before checkpoint
	if last <= latestCheckpoint {
		return -1, -1, nil
	}

	// there is gap between last checkpoint and segments
	if last > latestCheckpoint && latestCheckpoint < first-1 {
		return -1, -1, fmt.Errorf("gap between last checkpoint and segments")
	}

	return latestCheckpoint + 1, last, nil
}

// Checkpoint creates new checkpoint stopping at given segment
func (c *Checkpointer) Checkpoint(to int, targetWriter func() (io.WriteCloser, error)) (err error) {

	_, notCheckpointedTo, err := c.NotCheckpointedSegments()
	if err != nil {
		return fmt.Errorf("cannot get not checkpointed segments: %w", err)
	}

	latestCheckpoint, err := c.LatestCheckpoint()
	if err != nil {
		return fmt.Errorf("cannot get latest checkpoint: %w", err)
	}

	if latestCheckpoint == to {
		return nil //nothing to do
	}

	if notCheckpointedTo < to {
		return fmt.Errorf("no segments to checkpoint to %d, latests not checkpointed segment: %d", to, notCheckpointedTo)
	}

	forest, err := mtrie.NewForest(c.forestCapacity, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("cannot create Forest: %w", err)
	}

	err = c.wal.replay(0, to,
		func(tries []*trie.MTrie) error {
			return forest.AddTries(tries)
		},
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		}, func(rootHash ledger.RootHash) error {
			return nil
		}, true)

	if err != nil {
		return fmt.Errorf("cannot replay WAL: %w", err)
	}

	tries, err := forest.GetTries()
	if err != nil {
		return fmt.Errorf("cannot get forest tries: %w", err)
	}

	c.wal.log.Info().Msgf("serializing checkpoint %d", to)

	writer, err := targetWriter()
	if err != nil {
		return fmt.Errorf("cannot generate writer: %w", err)
	}
	defer func() {
		closeErr := writer.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	err = StoreCheckpoint(writer, tries...)

	return err
}

func NumberToFilenamePart(n int) string {
	return fmt.Sprintf("%08d", n)
}

func NumberToFilename(n int) string {

	return fmt.Sprintf("%s%s", checkpointFilenamePrefix, NumberToFilenamePart(n))
}

func (c *Checkpointer) CheckpointWriter(to int) (io.WriteCloser, error) {
	return CreateCheckpointWriter(c.dir, to)
}

func CreateCheckpointWriter(dir string, fileNo int) (io.WriteCloser, error) {
	return CreateCheckpointWriterForFile(dir, NumberToFilename(fileNo))
}

// CreateCheckpointWriterForFile returns a file writer that will write to a temporary file and then move it to the checkpoint folder by renaming it.
func CreateCheckpointWriterForFile(dir, filename string) (io.WriteCloser, error) {

	fullname := path.Join(dir, filename)

	if utilsio.FileExists(fullname) {
		return nil, fmt.Errorf("checkpoint file %s already exists", fullname)
	}

	tmpFile, err := ioutil.TempFile(dir, "writing-chkpnt-*")
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary file for checkpoint %v: %w", tmpFile, err)
	}

	writer := bufio.NewWriter(tmpFile)
	return &SyncOnCloseRenameFile{
		file:       tmpFile,
		targetName: fullname,
		Writer:     writer,
	}, nil
}

// StoreCheckpoint writes the given tries to checkpoint file, and also appends
// a CRC32 file checksum for integrity check.
// Checkpoint file consists of a flattened forest. Specifically, it consists of:
//   * a list of encoded nodes, where references to other nodes are by list index.
//   * a list of encoded tries, each referencing their respective root node by index.
// Referencing to other nodes by index 0 is a special case, meaning nil.
//
// As an important property, the nodes are listed in an order which satisfies
// Descendents-First-Relationship. The Descendents-First-Relationship has the
// following important property:
// When rebuilding the trie from the sequence of nodes, build the trie on the fly,
// as for each node, the children have been previously encountered.
// TODO: evaluate alternatives to CRC32 since checkpoint file is many GB in size.
// TODO: add concurrency if the performance gains are enough to offset complexity.
func StoreCheckpoint(writer io.Writer, tries ...*trie.MTrie) error {

	var err error

	crc32Writer := NewCRC32Writer(writer)

	// Write header: magic (2 bytes) + version (2 bytes)
	header := make([]byte, 4)
	pos := writeUint16(header, 0, MagicBytes)
	_ = writeUint16(header, pos, VersionV4)

	_, err = crc32Writer.Write(header)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint header: %w", err)
	}

	// assign unique index to every node
	allNodes := make(map[*node.Node]uint64)
	allNodes[nil] = 0 // 0th element is nil

	allRootNodes := make([]*node.Node, len(tries))

	// Serialize all unique nodes
	nodeCounter := uint64(1) // start from 1, as 0 marks nil
	for i, t := range tries {

		// Traverse all unique nodes for trie t.
		for itr := flattener.NewUniqueNodeIterator(t, allNodes); itr.Next(); {
			n := itr.Value()

			allNodes[n] = nodeCounter
			nodeCounter++

			var lchildIndex, rchildIndex uint64

			if lchild := n.LeftChild(); lchild != nil {
				var found bool
				lchildIndex, found = allNodes[lchild]
				if !found {
					hash := lchild.Hash()
					return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
				}
			}
			if rchild := n.RightChild(); rchild != nil {
				var found bool
				rchildIndex, found = allNodes[rchild]
				if !found {
					hash := rchild.Hash()
					return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
				}
			}

			// TODO: reuse scratch buffer for encoding
			bytes := flattener.EncodeNode(n, lchildIndex, rchildIndex)
			_, err = crc32Writer.Write(bytes)
			if err != nil {
				return fmt.Errorf("error while writing node data: %w", err)
			}
		}

		// Save trie root for serialization later.
		allRootNodes[i] = t.RootNode()
	}

	// Serialize trie root nodes
	for _, rootNode := range allRootNodes {
		// Get root node index
		rootIndex, found := allNodes[rootNode]
		if !found {
			var rootHash ledger.RootHash
			if rootNode == nil {
				rootHash = trie.EmptyTrieRootHash()
			} else {
				rootHash = ledger.RootHash(rootNode.Hash())
			}
			return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(rootHash[:]))
		}

		// TODO: reuse scratch buffer for encoding
		bytes := flattener.EncodeTrie(rootNode, rootIndex)
		_, err = crc32Writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing trie data: %w", err)
		}
	}

	// Write footer with nodes count and tries count
	footer := make([]byte, 10)
	pos = writeUint64(footer, 0, uint64(len(allNodes)-1)) // -1 to account for 0 node meaning nil
	writeUint16(footer, pos, uint16(len(allRootNodes)))

	_, err = crc32Writer.Write(footer)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint footer: %w", err)
	}

	// add CRC32 sum
	crc32buf := make([]byte, 4)
	writeUint32(crc32buf, 0, crc32Writer.Crc32())

	_, err = writer.Write(crc32buf)
	if err != nil {
		return fmt.Errorf("cannot write crc32: %w", err)
	}

	return nil
}

func (c *Checkpointer) LoadCheckpoint(checkpoint int) ([]*trie.MTrie, error) {
	filepath := path.Join(c.dir, NumberToFilename(checkpoint))
	return LoadCheckpoint(filepath)
}

func (c *Checkpointer) LoadRootCheckpoint() ([]*trie.MTrie, error) {
	filepath := path.Join(c.dir, bootstrap.FilenameWALRootCheckpoint)
	return LoadCheckpoint(filepath)
}

func (c *Checkpointer) HasRootCheckpoint() (bool, error) {
	if _, err := os.Stat(path.Join(c.dir, bootstrap.FilenameWALRootCheckpoint)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (c *Checkpointer) RemoveCheckpoint(checkpoint int) error {
	return os.Remove(path.Join(c.dir, NumberToFilename(checkpoint)))
}

func LoadCheckpoint(filepath string) ([]*trie.MTrie, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	return readCheckpoint(file)
}

func readCheckpoint(f *os.File) ([]*trie.MTrie, error) {
	// Read header: magic (2 bytes) + version (2 bytes)
	header := make([]byte, 4)
	_, err := io.ReadFull(f, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header bytes: %w", err)
	}

	magicBytes, pos := readUint16(header, 0)
	version, _ := readUint16(header, pos)

	if magicBytes != MagicBytes {
		return nil, fmt.Errorf("unknown file format. Magic constant %x does not match expected %x", magicBytes, MagicBytes)
	}

	// Reset offset
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	switch version {
	case VersionV1, VersionV3:
		return readCheckpointV3AndEarlier(f, version)
	case VersionV4:
		return readCheckpointV4(f)
	default:
		return nil, fmt.Errorf("unsupported file version %x ", version)
	}
}

// readCheckpointV3AndEarlier deserializes checkpoint file (version 3 and earlier) and returns a list of tries.
// Header (magic and version) are verified by the caller.
// TODO: return []*trie.MTrie directly without conversion to FlattenedForest.
func readCheckpointV3AndEarlier(f *os.File, version uint16) ([]*trie.MTrie, error) {

	var bufReader io.Reader = bufio.NewReader(f)
	crcReader := NewCRC32Reader(bufReader)

	var reader io.Reader

	if version != VersionV3 {
		reader = bufReader
	} else {
		reader = crcReader
	}

	// Header has: magic (2 bytes) + version (2 bytes) + node count (8 bytes) + trie count (2 bytes)
	header := make([]byte, 2+2+8+2)

	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header bytes: %w", err)
	}

	// Magic and version are verified by the caller.

	// Get node count and trie count
	const nodesCountOffset = 2 + 2
	nodesCount, pos := readUint64(header, nodesCountOffset)
	triesCount, _ := readUint16(header, pos)

	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		n, err := flattener.ReadNodeFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(i) {
				return nil, fmt.Errorf("sequence of stored nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = n
	}

	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrieFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(len(nodes)) {
				return nil, fmt.Errorf("sequence of stored nodes doesn't contain node")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read storable trie %d: %w", i, err)
		}
		tries[i] = trie
	}

	if version == VersionV3 {
		crc32buf := make([]byte, 4)
		_, err := bufReader.Read(crc32buf)
		if err != nil {
			return nil, fmt.Errorf("error while reading CRC32 checksum: %w", err)
		}
		readCrc32, _ := readUint32(crc32buf, 0)

		calculatedCrc32 := crcReader.Crc32()

		if calculatedCrc32 != readCrc32 {
			return nil, fmt.Errorf("checkpoint checksum failed! File contains %x but read data checksums to %x", readCrc32, calculatedCrc32)
		}
	}

	return tries, nil

}

// readCheckpointV4 deserializes checkpoint file (version 4) and returns a list of tries.
// Checkpoint file header (magic and version) are verified by the caller.
func readCheckpointV4(f *os.File) ([]*trie.MTrie, error) {

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// Read footer to get node count and trie count

	// footer offset: nodes count (8 bytes) + tries count (2 bytes) + CRC32 sum (4 bytes)
	const footerOffset = 8 + 2 + 4
	const footerSize = 8 + 2 // footer doesn't include crc32 sum

	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to footer: %w", err)
	}

	footer := scratch[:footerSize]

	_, err = io.ReadFull(f, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer bytes: %w", err)
	}

	nodesCount := binary.BigEndian.Uint64(footer)
	triesCount := binary.BigEndian.Uint16(footer[8:])

	// Seek to the start of file
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	var bufReader io.Reader = bufio.NewReader(f)
	crcReader := NewCRC32Reader(bufReader)
	var reader io.Reader = crcReader

	// Read header: magic (2 bytes) + version (2 bytes)
	// No action is needed for header because it is verified by the caller.

	_, err = io.ReadFull(reader, scratch[:4])
	if err != nil {
		return nil, fmt.Errorf("cannot read header bytes: %w", err)
	}

	// nodes's element at index 0 is a special, meaning nil .
	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		n, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(i) {
				return nil, fmt.Errorf("sequence of stored nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = n
	}

	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(len(nodes)) {
				return nil, fmt.Errorf("sequence of stored nodes doesn't contain node")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read storable trie %d: %w", i, err)
		}
		tries[i] = trie
	}

	// Read footer again for crc32 computation
	// No action is needed.
	_, err = io.ReadFull(reader, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer bytes: %w", err)
	}

	crc32buf := scratch[:4]
	_, err = bufReader.Read(crc32buf)
	if err != nil {
		return nil, fmt.Errorf("error while reading CRC32 checksum: %w", err)
	}
	readCrc32, _ := readUint32(crc32buf, 0)

	calculatedCrc32 := crcReader.Crc32()

	if calculatedCrc32 != readCrc32 {
		return nil, fmt.Errorf("checkpoint checksum failed! File contains %x but read data checksums to %x", readCrc32, calculatedCrc32)
	}

	return tries, nil
}

func writeUint16(buffer []byte, location int, value uint16) int {
	binary.BigEndian.PutUint16(buffer[location:], value)
	return location + 2
}

func readUint16(buffer []byte, location int) (uint16, int) {
	value := binary.BigEndian.Uint16(buffer[location:])
	return value, location + 2
}

func writeUint32(buffer []byte, location int, value uint32) int {
	binary.BigEndian.PutUint32(buffer[location:], value)
	return location + 4
}

func readUint32(buffer []byte, location int) (uint32, int) {
	value := binary.BigEndian.Uint32(buffer[location:])
	return value, location + 4
}

func readUint64(buffer []byte, location int) (uint64, int) {
	value := binary.BigEndian.Uint64(buffer[location:])
	return value, location + 8
}

func writeUint64(buffer []byte, location int, value uint64) int {
	binary.BigEndian.PutUint64(buffer[location:], value)
	return location + 8
}
