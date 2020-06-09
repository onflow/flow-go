package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/prometheus/tsdb/fileutil"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/sequencer"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
)

var checkpointFilenamePrefix = "checkpoint."

type Checkpointer struct {
	dir       string
	wal       *LedgerWAL
	maxHeight int
	cacheSize int
}

func NewCheckpointer(wal *LedgerWAL, maxHeight int, cacheSize int) *Checkpointer {
	return &Checkpointer{
		dir:       wal.wal.Dir(),
		wal:       wal,
		maxHeight: maxHeight,
		cacheSize: cacheSize,
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

	mForest, err := mtrie.NewMForest(c.maxHeight, c.dir, c.cacheSize, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("cannot create mForest: %w", err)
	}

	err = c.wal.replay(0, to,
		func(forestSequencing *sequencer.MForestSequencing) error {
			tries, err := sequencer.RebuildTries(forestSequencing)
			if err != nil {
				return err
			}
			for _, t := range tries {
				err := mForest.AddTrie(t)
				if err != nil {
					return err
				}
			}
			return nil
		},
		func(commitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			_, err := mForest.Update(commitment, keys, values)
			return err
		}, func(commitment flow.StateCommitment) error {
			return nil
		})

	if err != nil {
		return fmt.Errorf("cannot replay WAL: %w", err)
	}

	fmt.Printf("Got the tries...\n")

	forestSequencing, err := sequencer.SequenceForest(mForest)
	if err != nil {
		return fmt.Errorf("cannot get storables: %w", err)
	}

	writer, err := targetWriter()
	if err != nil {
		return fmt.Errorf("cannot generate writer: %w", err)
	}
	defer writer.Close()

	err = c.StoreCheckpoint(forestSequencing, writer)

	return err
}

func numberToFilenamePart(n int) string {
	return fmt.Sprintf("%08d", n)
}

func numberToFilename(n int) string {

	return fmt.Sprintf("%s%s", checkpointFilenamePrefix, numberToFilenamePart(n))
}

type SyncOnCloseFile struct {
	file *os.File
	*bufio.Writer
}

func (s *SyncOnCloseFile) Close() error {
	defer func() {
		_ = s.file.Close()
	}()

	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}

	return s.file.Sync()
}

func (c *Checkpointer) CheckpointWriter(to int) (io.WriteCloser, error) {
	file, err := os.Create(path.Join(c.dir, numberToFilename(to)))
	if err != nil {
		return nil, fmt.Errorf("cannot create file for checkpoint %d: %w", to, err)
	}

	writer := bufio.NewWriter(file)
	return &SyncOnCloseFile{
		file:   file,
		Writer: writer,
	}, nil
}

func (c *Checkpointer) StoreCheckpoint(forestSequencing *sequencer.MForestSequencing, writer io.WriteCloser) error {
	storableNodes := forestSequencing.Nodes
	storableTries := forestSequencing.Tries
	header := make([]byte, 8+2)

	pos := writeUint64(header, 0, uint64(len(storableNodes)-1)) // -1 to account for 0 node meaning nil
	writeUint16(header, pos, uint16(len(storableTries)))

	_, err := writer.Write(header)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint header: %w", err)
	}

	// 0 element = nil, we don't need to store it
	for i := 1; i < len(storableNodes); i++ {
		bytes := EncodeStorableNode(storableNodes[i])
		_, err = writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing node date: %w", err)
		}
	}

	for _, storableTrie := range storableTries {
		bytes := EncodeStorableTrie(storableTrie)
		_, err = writer.Write(bytes)
		if err != nil {
			return fmt.Errorf("error while writing trie date: %w", err)
		}
	}

	return nil
}

func (c *Checkpointer) LoadCheckpoint(checkpoint int) (*sequencer.MForestSequencing, error) {

	filepath := path.Join(c.dir, numberToFilename(checkpoint))
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)

	header := make([]byte, 8+2)

	_, err = io.ReadFull(reader, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header bytes: %w", err)
	}

	nodesCount, pos := readUint64(header, 0)
	triesCount, _ := readUint16(header, pos)

	nodes := make([]*sequencer.StorableNode, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*sequencer.StorableTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		storableNode, err := ReadStorableNode(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read storable node %d: %w", i, err)
		}
		nodes[i] = storableNode
	}

	for i := uint16(0); i < triesCount; i++ {
		storableTrie, err := ReadStorableTrie(reader)
		if err != nil {
			return nil, fmt.Errorf("cannot read storable trie %d: %w", i, err)
		}
		tries[i] = storableTrie
	}

	return &sequencer.MForestSequencing{
		Nodes: nodes,
		Tries: tries,
	}, nil

}
