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
func (c *Checkpointer) Checkpoint(to int, targetWriter func() (io.WriteCloser, error)) error {

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
		func(forestSequencing *flattener.FlattenedForest) error {
			tries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return err
			}
			for _, t := range tries {
				err := forest.AddTrie(t)
				if err != nil {
					return err
				}
			}
			return nil
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

	forestSequencing, err := flattener.FlattenForest(forest)
	if err != nil {
		return fmt.Errorf("cannot get storables: %w", err)
	}

	writer, err := targetWriter()
	if err != nil {
		return fmt.Errorf("cannot generate writer: %w", err)
	}
	defer writer.Close()

	err = StoreCheckpoint(forestSequencing, writer)

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

// StoreCheckpoint writes the given checkpoint to disk, and also append with a CRC32 file checksum for integrity check.
func StoreCheckpoint(forestSequencing *flattener.FlattenedForest, writer io.Writer) error {
	storableNodes := forestSequencing.Nodes
	storableTries := forestSequencing.Tries
	header := make([]byte, 4+8+2)

	crc32Writer := NewCRC32Writer(writer)

	pos := writeUint16(header, 0, MagicBytes)
	pos = writeUint16(header, pos, VersionV3)
	pos = writeUint64(header, pos, uint64(len(storableNodes)-1)) // -1 to account for 0 node meaning nil
	writeUint16(header, pos, uint16(len(storableTries)))

	_, err := crc32Writer.Write(header)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint header: %w", err)
	}

	// 0 element = nil, we don't need to store it
	for i := 1; i < len(storableNodes); i++ {
		bytes := flattener.EncodeStorableNode(storableNodes[i])
		_, err = crc32Writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing node date: %w", err)
		}
	}

	for _, storableTrie := range storableTries {
		bytes := flattener.EncodeStorableTrie(storableTrie)
		_, err = crc32Writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing trie date: %w", err)
		}
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

func (c *Checkpointer) LoadCheckpoint(checkpoint int) (*flattener.FlattenedForest, error) {
	filepath := path.Join(c.dir, NumberToFilename(checkpoint))
	return LoadCheckpoint(filepath)
}

func (c *Checkpointer) LoadRootCheckpoint() (*flattener.FlattenedForest, error) {
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

func LoadCheckpoint(filepath string) (*flattener.FlattenedForest, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	return ReadCheckpoint(file)
}

func ReadCheckpoint(r io.Reader) (*flattener.FlattenedForest, error) {

	var bufReader io.Reader = bufio.NewReader(r)
	crcReader := NewCRC32Reader(bufReader)
	var reader io.Reader = crcReader

	header := make([]byte, 4+8+2)

	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header bytes: %w", err)
	}

	magicBytes, pos := readUint16(header, 0)
	version, pos := readUint16(header, pos)
	nodesCount, pos := readUint64(header, pos)
	triesCount, _ := readUint16(header, pos)

	if magicBytes != MagicBytes {
		return nil, fmt.Errorf("unknown file format. Magic constant %x does not match expected %x", magicBytes, MagicBytes)
	}
	if version != VersionV1 && version != VersionV3 {
		return nil, fmt.Errorf("unsupported file version %x ", version)
	}

	if version != VersionV3 {
		reader = bufReader //switch back to plain reader
	}

	nodes := make([]*flattener.StorableNode, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*flattener.StorableTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		storableNode, err := flattener.ReadStorableNode(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read storable node %d: %w", i, err)
		}
		nodes[i] = storableNode
	}

	// TODO version ?
	for i := uint16(0); i < triesCount; i++ {
		storableTrie, err := flattener.ReadStorableTrie(reader)
		fmt.Printf(">>>> loading trie from checkpoint with root hash %s \n", hex.EncodeToString(storableTrie.RootHash))
		if err != nil {
			return nil, fmt.Errorf("cannot read storable trie %d: %w", i, err)
		}
		tries[i] = storableTrie
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

	return &flattener.FlattenedForest{
		Nodes: nodes,
		Tries: tries,
	}, nil

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
