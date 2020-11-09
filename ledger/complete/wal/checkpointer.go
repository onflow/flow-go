package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/prometheus/tsdb/fileutil"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/metrics"
)

const checkpointFilenamePrefix = "checkpoint."

const MagicBytes uint16 = 0x2137
const VersionV1 uint16 = 0x01
const VersionV2 uint16 = 0x02

const RootCheckpointFilename = "root.checkpoint"

type Checkpointer struct {
	dir            string
	wal            *LedgerWAL
	keyByteSize    int
	forestCapacity int
}

func NewCheckpointer(wal *LedgerWAL, keyByteSize int, forestCapacity int) *Checkpointer {
	return &Checkpointer{
		dir:            wal.wal.Dir(),
		wal:            wal,
		keyByteSize:    keyByteSize,
		forestCapacity: forestCapacity,
	}
}

// LatestCheckpoint returns number of latest checkpoint or -1 if there are no checkpoints
func (c *Checkpointer) LatestCheckpoint() (int, error) {

	files, err := fileutil.ReadDir(c.dir)
	if err != nil {
		return -1, err
	}
	last := -1
	for _, fn := range files {
		if !strings.HasPrefix(fn, checkpointFilenamePrefix) {
			continue
		}
		justNumber := fn[len(checkpointFilenamePrefix):]
		k, err := strconv.Atoi(justNumber)
		if err != nil {
			continue
		}

		last = k
	}

	return last, nil
}

// NotCheckpointedSegments - returns numbers of segments which are not checkpointed yet,
// or -1, -1 if there are no segments
func (c *Checkpointer) NotCheckpointedSegments() (from, to int, err error) {

	latestCheckpoint, err := c.LatestCheckpoint()
	if err != nil {
		return -1, -1, fmt.Errorf("cannot get last checkpoint: %w", err)
	}

	first, last, err := c.wal.wal.Segments()
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

	forest, err := mtrie.NewForest(c.keyByteSize, c.dir, c.forestCapacity, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error {
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

type SyncOnCloseFile struct {
	file *os.File
	*bufio.Writer
}

func (s *SyncOnCloseFile) Sync() error {
	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}
	return s.file.Sync()

}

func (s *SyncOnCloseFile) Close() error {
	defer func() {
		err := s.file.Close()
		if err != nil {
			fmt.Printf("error while closing file: %s", err)
		}
	}()

	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}
	return s.file.Sync()
}

func (c *Checkpointer) CheckpointWriter(to int) (io.WriteCloser, error) {
	return CreateCheckpointWriter(c.dir, to)
}

func CreateCheckpointWriter(dir string, fileNo int) (io.WriteCloser, error) {
	filename := path.Join(dir, NumberToFilename(fileNo))
	return CreateCheckpointWriterForFile(filename)
}

func CreateCheckpointWriterForFile(filename string) (io.WriteCloser, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot create file for checkpoint %s: %w", filename, err)
	}

	writer := bufio.NewWriter(file)
	return &SyncOnCloseFile{
		file:   file,
		Writer: writer,
	}, nil
}

func StoreCheckpoint(forestSequencing *flattener.FlattenedForest, writer io.WriteCloser) error {
	storableNodes := forestSequencing.Nodes
	storableTries := forestSequencing.Tries
	header := make([]byte, 4+8+2)

	pos := writeUint16(header, 0, MagicBytes)
	pos = writeUint16(header, pos, VersionV1)
	pos = writeUint64(header, pos, uint64(len(storableNodes)-1)) // -1 to account for 0 node meaning nil
	writeUint16(header, pos, uint16(len(storableTries)))

	_, err := writer.Write(header)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint header: %w", err)
	}

	// 0 element = nil, we don't need to store it
	for i := 1; i < len(storableNodes); i++ {
		bytes := flattener.EncodeStorableNode(storableNodes[i])
		_, err = writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing node date: %w", err)
		}
	}

	for _, storableTrie := range storableTries {
		bytes := flattener.EncodeStorableTrie(storableTrie)
		_, err = writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing trie date: %w", err)
		}
	}

	return nil
}

func (c *Checkpointer) LoadCheckpoint(checkpoint int) (*flattener.FlattenedForest, error) {
	filepath := path.Join(c.dir, NumberToFilename(checkpoint))
	return LoadCheckpoint(filepath)
}

func (c *Checkpointer) LoadRootCheckpoint() (*flattener.FlattenedForest, error) {
	filepath := path.Join(c.dir, RootCheckpointFilename)
	return LoadCheckpoint(filepath)
}

func (c *Checkpointer) HasRootCheckpoint() (bool, error) {
	if _, err := os.Stat(path.Join(c.dir, RootCheckpointFilename)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func LoadCheckpoint(filepath string) (*flattener.FlattenedForest, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)

	header := make([]byte, 4+8+2)

	_, err = io.ReadFull(reader, header)
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
	if version != VersionV1 && version != VersionV2 {
		return nil, fmt.Errorf("unsupported file version %x ", version)
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
		if err != nil {
			return nil, fmt.Errorf("cannot read storable trie %d: %w", i, err)
		}
		tries[i] = storableTrie
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

func readUint64(buffer []byte, location int) (uint64, int) {
	value := binary.BigEndian.Uint64(buffer[location:])
	return value, location + 8
}

func writeUint64(buffer []byte, location int, value uint64) int {
	binary.BigEndian.PutUint64(buffer[location:], value)
	return location + 8
}
