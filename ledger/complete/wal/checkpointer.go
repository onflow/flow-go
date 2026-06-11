package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	utilsio "github.com/onflow/flow-go/utils/io"
)

const checkpointFilenamePrefix = "checkpoint."

const (
	MagicBytesCheckpointHeader  uint16 = 0x2137
	MagicBytesCheckpointSubtrie uint16 = 0x2136
	MagicBytesCheckpointToptrie uint16 = 0x2135
	MagicBytesPayloadHeader     uint16 = 0x2138
)

const VersionV1 uint16 = 0x01

// Versions was reset while changing trie format, so now bump it to 3 to avoid conflicts
// Version 3 contains a file checksum for detecting corrupted checkpoint files.
const VersionV3 uint16 = 0x03

// Version 4 contains a footer with node count and trie count (previously in the header).
// Version 4 also reduces checkpoint data size.  See EncodeNode() and EncodeTrie() for more details.
const VersionV4 uint16 = 0x04

// Version 5 includes these changes:
// - remove regCount and maxDepth from serialized nodes
// - add allocated register count and size to serialized tries
// - reduce number of bytes used to encode payload value size from 8 bytes to 4 bytes.
// See EncodeNode() and EncodeTrie() for more details.
const VersionV5 uint16 = 0x05

// Version 6 includes these changes:
//   - trie nodes are stored in additional 17 checkpoint files, with .0, .1, .2, ... .16 as
//     file name extension
const VersionV6 uint16 = 0x06

// Version 7 includes these changes:
//   - payloadless mode: leaf nodes store payload hashes (32 bytes) instead of full payloads
//   - used for verification nodes that don't need actual payload values
const VersionV7 uint16 = 0x07

// MaxVersion is the latest checkpoint version we support.
// Need to update MaxVersion when creating a newer version.
const MaxVersion = VersionV7

// V7FileSuffix is appended to V7 (payloadless) checkpoint filenames so they are
// visibly distinct from V6 files and can coexist with them in the same directory.
// Example: V6 = "checkpoint.00000100", V7 = "checkpoint.00000100.v7"
const V7FileSuffix = ".v7"

// CheckpointInfo contains metadata about a checkpoint file parsed from its filename.
type CheckpointInfo struct {
	Number  int    // Checkpoint number (e.g., 100 for "checkpoint.00000100")
	Version uint16 // Checkpoint version (VersionV6 or VersionV7)
}

const (
	encMagicSize        = 2
	encVersionSize      = 2
	headerSize          = encMagicSize + encVersionSize
	encSubtrieCountSize = 2
	encNodeCountSize    = 8
	encTrieCountSize    = 2
	crc32SumSize        = 4
)

// defaultBufioReadSize replaces the default bufio buffer size of 4096 bytes.
// defaultBufioReadSize can be increased to 8KiB, 16KiB, 32KiB, etc. if it
// improves performance on typical EN hardware.
const defaultBufioReadSize = 1024 * 32

// defaultBufioWriteSize replaces the default bufio buffer size of 4096 bytes.
// defaultBufioWriteSize can be increased to 8KiB, 16KiB, 32KiB, etc. if it
// improves performance on typical EN hardware.
const defaultBufioWriteSize = 1024 * 32

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

// listV6Checkpoints returns V6 checkpoint numbers (unsorted) and the last V6 number.
// This Checkpointer writes V6 only, so its scheduling decisions (LatestCheckpointV6,
// NotCheckpointedSegments, the Checkpoint(to) no-op short-circuit) must track V6
// progress to avoid being misled by stray V7 files dropped in the same directory.
// For cross-version inspection, use the package-level ListCheckpoints or
// ListV7Checkpoints functions.
func (c *Checkpointer) listV6Checkpoints() ([]int, int, error) {
	return ListV6Checkpoints(c.dir)
}

// ListCheckpoints returns all the numbers of the checkpoint files (both V6 and V7), and the number of the last checkpoint.
// Note: it doesn't include the root checkpoint file.
// For version-specific listing, use ListV6Checkpoints or ListV7Checkpoints.
func ListCheckpoints(dir string) ([]int, int, error) {
	infos, lastInfo, err := ListCheckpointsWithInfo(dir)
	if err != nil {
		return nil, -1, err
	}

	// Deduplicate by number (a checkpoint number may have both V6 and V7)
	seen := make(map[int]struct{})
	list := make([]int, 0, len(infos))
	for _, info := range infos {
		if _, exists := seen[info.Number]; !exists {
			seen[info.Number] = struct{}{}
			list = append(list, info.Number)
		}
	}

	last := -1
	if lastInfo != nil {
		last = lastInfo.Number
	}

	return list, last, nil
}

// ListCheckpointsWithInfo returns all checkpoint infos and the latest checkpoint info.
// It detects both V6 and V7 checkpoints based on their filenames.
// Note: it doesn't include the root checkpoint file.
func ListCheckpointsWithInfo(dir string) ([]CheckpointInfo, *CheckpointInfo, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot list directory [%s] content: %w", dir, err)
	}

	list := make([]CheckpointInfo, 0)
	var last *CheckpointInfo

	for _, fn := range files {
		info, ok := parseCheckpointFilename(fn.Name())
		if !ok {
			continue
		}

		list = append(list, info)

		// Track the latest checkpoint (highest number; V7 takes precedence over V6 for same number)
		if last == nil || info.Number > last.Number ||
			(info.Number == last.Number && info.Version > last.Version) {
			infoCopy := info
			last = &infoCopy
		}
	}

	return list, last, nil
}

// ListV6Checkpoints returns all V6 checkpoint numbers (unsorted) and the latest V6 checkpoint number.
// Returns -1 as the latest if no V6 checkpoints exist.
func ListV6Checkpoints(dir string) ([]int, int, error) {
	infos, _, err := ListCheckpointsWithInfo(dir)
	if err != nil {
		return nil, -1, err
	}

	list := make([]int, 0)
	last := -1
	for _, info := range infos {
		if info.Version == VersionV6 {
			list = append(list, info.Number)
			if info.Number > last {
				last = info.Number
			}
		}
	}
	return list, last, nil
}

// ListV7Checkpoints returns all V7 checkpoint numbers (unsorted) and the latest V7 checkpoint number.
// Returns -1 as the latest if no V7 checkpoints exist.
func ListV7Checkpoints(dir string) ([]int, int, error) {
	infos, _, err := ListCheckpointsWithInfo(dir)
	if err != nil {
		return nil, -1, err
	}

	list := make([]int, 0)
	last := -1
	for _, info := range infos {
		if info.Version == VersionV7 {
			list = append(list, info.Number)
			if info.Number > last {
				last = info.Number
			}
		}
	}
	return list, last, nil
}

// Checkpoints returns all the numbers of the checkpoint files in asc order.
// note, it doesn't include the root checkpoint file
func (c *Checkpointer) Checkpoints() ([]int, error) {
	return Checkpoints(c.dir)
}

// Checkpoints returns all the checkpoint numbers in asc order
func Checkpoints(dir string) ([]int, error) {
	list, _, err := ListCheckpoints(dir)
	if err != nil {
		return nil, fmt.Errorf("could not fetch all checkpoints: %w", err)
	}

	sort.Ints(list)

	return list, nil
}

// CheckpointsV6 returns all V6 checkpoint numbers in asc order.
// Use this when loading checkpoints in non-payloadless mode.
func (c *Checkpointer) CheckpointsV6() ([]int, error) {
	list, _, err := ListV6Checkpoints(c.dir)
	if err != nil {
		return nil, fmt.Errorf("could not fetch V6 checkpoints: %w", err)
	}
	sort.Ints(list)
	return list, nil
}

// CheckpointsV7 returns all V7 checkpoint numbers in asc order.
// Use this when loading checkpoints in payloadless mode.
func (c *Checkpointer) CheckpointsV7() ([]int, error) {
	list, _, err := ListV7Checkpoints(c.dir)
	if err != nil {
		return nil, fmt.Errorf("could not fetch V7 checkpoints: %w", err)
	}
	sort.Ints(list)
	return list, nil
}

// LatestCheckpointV6 returns the number of the latest V6 checkpoint, or -1 if
// there are no V6 checkpoints. V7 (payloadless) files in the same directory are
// ignored — see [Checkpointer.listV6Checkpoints] for rationale.
func (c *Checkpointer) LatestCheckpointV6() (int, error) {
	_, last, err := c.listV6Checkpoints()
	return last, err
}

// NotCheckpointedSegments - returns numbers of segments which are not checkpointed yet,
// or -1, -1 if there are no segments
func (c *Checkpointer) NotCheckpointedSegments() (from, to int, err error) {

	latestCheckpoint, err := c.LatestCheckpointV6()
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
func (c *Checkpointer) Checkpoint(to int) (err error) {

	_, notCheckpointedTo, err := c.NotCheckpointedSegments()
	if err != nil {
		return fmt.Errorf("cannot get not checkpointed segments: %w", err)
	}

	latestCheckpoint, err := c.LatestCheckpointV6()
	if err != nil {
		return fmt.Errorf("cannot get latest checkpoint: %w", err)
	}

	if latestCheckpoint == to {
		return nil //nothing to do
	}

	if notCheckpointedTo < to {
		return fmt.Errorf("no segments to checkpoint to %d, latests not checkpointed segment: %d", to, notCheckpointedTo)
	}

	forest, err := mtrie.NewForest(c.forestCapacity, &metrics.NoopCollector{}, nil)
	if err != nil {
		return fmt.Errorf("cannot create Forest: %w", err)
	}

	c.wal.log.Info().Msgf("creating checkpoint %d", to)

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

	// The standard Checkpointer replays the WAL into a regular [mtrie.Forest], which
	// only produces full (V6) tries. Payloadless (V7) checkpoints are generated by a
	// separate code path that operates on a [payloadless.Forest] and calls
	// [StoreCheckpointV7*] directly with []*payloadless.MTrie.
	fileName := NumberToFilename(to)
	err = StoreCheckpointV6SingleThread(tries, c.wal.dir, fileName, c.wal.log)

	if err != nil {
		return fmt.Errorf("could not create checkpoint for %v: %w", to, err)
	}

	checkpointFileSize, err := ReadCheckpointFileSize(c.wal.dir, fileName)
	if err != nil {
		return fmt.Errorf("could not read checkpoint file size: %w", err)
	}

	c.wal.log.Info().
		Str("checkpoint_file_size", units.BytesSize(float64(checkpointFileSize))).
		Msgf("created checkpoint %d with %d tries", to, len(tries))

	return nil
}

func NumberToFilenamePart(n int) string {
	return fmt.Sprintf("%08d", n)
}

func NumberToFilename(n int) string {
	return fmt.Sprintf("%s%s", checkpointFilenamePrefix, NumberToFilenamePart(n))
}

// NumberToFilenameV7 returns the V7 (payloadless) checkpoint filename for a given number.
// Example: 100 -> "checkpoint.00000100.v7"
func NumberToFilenameV7(n int) string {
	return fmt.Sprintf("%s%s%s", checkpointFilenamePrefix, NumberToFilenamePart(n), V7FileSuffix)
}

// parseCheckpointFilename parses a checkpoint filename and returns its info.
// Returns (info, true) if successful, (CheckpointInfo{}, false) otherwise.
//
// Handles:
//   - "checkpoint.00000100"     -> {100, VersionV6}
//   - "checkpoint.00000100.v7"  -> {100, VersionV7}
//
// Does NOT match part files like "checkpoint.00000100.001" or
// "checkpoint.00000100.v7.001".
func parseCheckpointFilename(fname string) (CheckpointInfo, bool) {
	if !strings.HasPrefix(fname, checkpointFilenamePrefix) {
		return CheckpointInfo{}, false
	}

	// Remove prefix: "checkpoint.00000100" -> "00000100" or "00000100.v7"
	suffix := fname[len(checkpointFilenamePrefix):]

	// Check for V7 suffix
	if strings.HasSuffix(suffix, V7FileSuffix) {
		numStr := suffix[:len(suffix)-len(V7FileSuffix)]
		// Must be exactly 8 digits
		if len(numStr) != 8 {
			return CheckpointInfo{}, false
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return CheckpointInfo{}, false
		}
		return CheckpointInfo{Number: n, Version: VersionV7}, true
	}

	// Try to parse as V6 - must be exactly 8 digits
	// This distinguishes "checkpoint.00000100" (V6 header) from "checkpoint.00000100.001" (part file)
	if len(suffix) != 8 {
		return CheckpointInfo{}, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return CheckpointInfo{}, false
	}
	return CheckpointInfo{Number: n, Version: VersionV6}, true
}

func (c *Checkpointer) CheckpointWriter(to int) (io.WriteCloser, error) {
	return CreateCheckpointWriterForFile(c.dir, NumberToFilename(to), c.wal.log)
}

func (c *Checkpointer) Dir() string {
	return c.dir
}

// CreateCheckpointWriterForFile returns a file writer that will write to a temporary file and then move it to the checkpoint folder by renaming it.
func CreateCheckpointWriterForFile(dir, filename string, logger zerolog.Logger) (io.WriteCloser, error) {

	fullname := path.Join(dir, filename)

	if utilsio.FileExists(fullname) {
		return nil, fmt.Errorf("checkpoint file %s already exists", fullname)
	}

	tmpFile, err := os.CreateTemp(dir, "writing-chkpnt-*")
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary file for checkpoint %v: %w", tmpFile, err)
	}

	writer := bufio.NewWriterSize(tmpFile, defaultBufioWriteSize)
	return &SyncOnCloseRenameFile{
		logger:     logger,
		file:       tmpFile,
		targetName: fullname,
		Writer:     writer,
	}, nil
}

// StoreCheckpointV5 writes the given tries to checkpoint file, and also appends
// a CRC32 file checksum for integrity check.
// Checkpoint file consists of a flattened forest. Specifically, it consists of:
//   - a list of encoded nodes, where references to other nodes are by list index.
//   - a list of encoded tries, each referencing their respective root node by index.
//
// Referencing to other nodes by index 0 is a special case, meaning nil.
//
// As an important property, the nodes are listed in an order which satisfies
// Descendents-First-Relationship. The Descendents-First-Relationship has the
// following important property:
// When rebuilding the trie from the sequence of nodes, build the trie on the fly,
// as for each node, the children have been previously encountered.
// TODO: evaluate alternatives to CRC32 since checkpoint file is many GB in size.
// TODO: add concurrency if the performance gains are enough to offset complexity.
func StoreCheckpointV5(dir string, fileName string, logger zerolog.Logger, tries ...*trie.MTrie) (
	// error
	// Note, the above code, which didn't define the name "err" for the returned error, would be wrong,
	// beause err needs to be defined in order to be updated by the defer function
	errToReturn error,
) {
	writer, err := CreateCheckpointWriterForFile(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not create writer: %w", err)
	}
	defer func() {
		errToReturn = closeAndMergeError(writer, errToReturn)
	}()

	crc32Writer := NewCRC32Writer(writer)

	// Scratch buffer is used as temporary buffer that node can encode into.
	// Data in scratch buffer should be copied or used before scratch buffer is used again.
	// If the scratch buffer isn't large enough, a new buffer will be allocated.
	// However, 4096 bytes will be large enough to handle almost all payloads
	// and 100% of interim nodes.
	scratch := make([]byte, 1024*4)

	// Write header: magic (2 bytes) + version (2 bytes)
	header := scratch[:headerSize]
	binary.BigEndian.PutUint16(header, MagicBytesCheckpointHeader)
	binary.BigEndian.PutUint16(header[encMagicSize:], VersionV5)

	_, err = crc32Writer.Write(header)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint header: %w", err)
	}

	// Multiple tries might have shared nodes at higher level, However, we don't want to
	// seralize duplicated nodes in the checkpoint file. In order to deduplicate, we build
	// a map from unique nodes while iterating and seralizing the nodes to the checkpoint file.
	//
	// The map for deduplication contains all the trie nodes, which uses a lot of memory.
	// In fact, we don't have to build a map for all nodes, since there are nodes which
	// are never shared.  Nodes can only be shared if and only if they are
	// on the same path. In other words, nodes on different path won't be shared.
	// If we group trie nodes by path, then we have more smaller groups of trie nodes from the same path,
	// which might have duplication. And then for each group, we could build a smaller map for deduplication.
	// Processing each group sequentially would allow us reduce operational memory.
	//
	// With this idea in mind, the seralization can be done in two steps:
	// 1. serialize nodes in subtries (tries with root at subtrieLevel).
	// 2. serialize remaining nodes (from trie root to subtrie root).
	// For instance, if there are 3 top tries, and subtrieLevel is 4, then there will be
	// 	(2 ^ 4) * 3 = 48 subtrie root nodes at level 4.
	// Then step 1 will seralize the 48 subtrie root nodes into the checkpoint file, and
	// then step 2 will seralize the 3 root nodes (level 0) and the interim nodes from level 1 to 3 into
	//
	// Step 1:
	// 1. Find all the subtrie root nodes at subtrieLevel (level 4)
	// 2. Group the subtrie by path. Since subtries in different group have different path, they won't have
	//		child nodes shared. Subtries in the same group might have duplication, we will build a map to deduplicate.
	//
	// subtrieLevel is number of edges from trie root to subtrie root.
	// Trie root is at level 0.
	const subtrieLevel = 4

	// subtrieCount is number of subtries at subtrieLevel.
	const subtrieCount = 1 << subtrieLevel

	// since each trie has `subtrieCount` number of subtries at subtrieLevel,
	// we create `subtrieCount` number of groups, each group contains all the subtrie root nodes

	// subtrieRoots is an array of groups.
	// Each group contains the subtrie roots of the same path at subtrieLevel for different tries.
	// For example, if subtrieLevel is 4, then
	// - subtrieRoots[0] is a list of all subtrie roots at path [0,0,0,0]
	// - subtrieRoots[1] is a list of all subtrie roots at path [0,0,0,1]
	// - subtrieRoots[subtrieCount-1] is a list of all subtrie roots at path [1,1,1,1]
	// subtrie roots in subtrieRoots[0] have the same path, therefore might have shared child nodes.
	var subtrieRoots [subtrieCount][]*node.Node
	for i := 0; i < len(subtrieRoots); i++ {
		subtrieRoots[i] = make([]*node.Node, len(tries))
	}

	for trieIndex, t := range tries {
		// subtries is an array with subtrieCount trie nodes
		// in breadth-first order at subtrieLevel of the trie `t`
		subtries := getNodesAtLevel(t.RootNode(), subtrieLevel)
		for subtrieIndex, subtrieRoot := range subtries {
			subtrieRoots[subtrieIndex][trieIndex] = subtrieRoot
		}
	}

	// topLevelNodes contains all unique nodes of given tries
	// from root to subtrie root and their index
	// (ordered by node traversal sequence).
	// Index 0 is a special case with nil node.
	topLevelNodes := make(map[*node.Node]uint64, 1<<(subtrieLevel+1))
	topLevelNodes[nil] = 0

	// nodeCounter is counter for all unique nodes.
	// It starts from 1, as 0 marks nil node.
	nodeCounter := uint64(1)

	// estimatedSubtrieNodeCount is rough estimate of number of nodes in subtrie,
	// assuming trie is a full binary tree.  estimatedSubtrieNodeCount is used
	// to preallocate traversedSubtrieNodes for memory efficiency.
	estimatedSubtrieNodeCount := 0
	if len(tries) > 0 {
		estimatedTrieNodeCount := 2*int(tries[0].AllocatedRegCount()) - 1
		estimatedSubtrieNodeCount = estimatedTrieNodeCount / subtrieCount
	}

	// Serialize subtrie nodes
	for i, subTrieRoot := range subtrieRoots {
		// traversedSubtrieNodes contains all unique nodes of subtries of the same path and their index.
		traversedSubtrieNodes := make(map[*node.Node]uint64, estimatedSubtrieNodeCount)
		// Index 0 is a special case with nil node.
		traversedSubtrieNodes[nil] = 0

		logging := logProgress(fmt.Sprintf("storing %v-th sub trie roots", i), estimatedSubtrieNodeCount, logger)
		for _, root := range subTrieRoot {
			// Empty trie is always added to forest as starting point and
			// empty trie's root is nil. It remains in the forest until evicted
			// by trie queue exceeding capacity.
			if root == nil {
				continue
			}
			// Note: nodeCounter is to assign an global index to each node in the order of it being seralized
			// into the checkpoint file. Therefore, it has to be reused when iterating each subtrie.
			// storeUniqueNodes will add the unique visited node into traversedSubtrieNodes with key as the node
			// itself, and value as n-th node being seralized in the checkpoint file.
			nodeCounter, err = storeUniqueNodes(root, traversedSubtrieNodes, nodeCounter, scratch, crc32Writer, logging)
			if err != nil {
				return fmt.Errorf("fail to store nodes in step 1 for subtrie root %v: %w", root.Hash(), err)
			}
			// Save subtrie root node index in topLevelNodes,
			// so when traversing top level tries
			// (from level 0 to subtrieLevel) using topLevelNodes,
			// node iterator skips subtrie as visited nodes.
			topLevelNodes[root] = traversedSubtrieNodes[root]
		}
	}

	// Step 2:
	// Now all nodes above and include the subtrieLevel have been seralized. We now
	// serialize remaining nodes of each trie from root node (level 0) to (subtrieLevel - 1).
	for _, t := range tries {
		root := t.RootNode()
		if root == nil {
			continue
		}
		// if we iterate through the root trie with an empty visited nodes map, then it will iterate through
		// all nodes at all levels. In order to skip the nodes above subtrieLevel, since they have been seralized in step 1,
		// we will need to pass in a visited nodes map that contains all the subtrie root nodes, which is the topLevelNodes.
		// The topLevelNodes was built in step 1, when seralizing each subtrie root nodes.
		nodeCounter, err = storeUniqueNodes(root, topLevelNodes, nodeCounter, scratch, crc32Writer, func(uint64) {})
		if err != nil {
			return fmt.Errorf("fail to store nodes in step 2 for root trie %v: %w", root.Hash(), err)
		}
	}

	// The root tries are seralized at the end of the checkpoint file, so that it's easy to find what tries are
	// included.
	for _, t := range tries {
		rootNode := t.RootNode()
		if !t.IsEmpty() && rootNode.Height() != ledger.NodeMaxHeight {
			return fmt.Errorf("height of root node must be %d, but is %d",
				ledger.NodeMaxHeight, rootNode.Height())
		}

		// Get root node index
		rootIndex, found := topLevelNodes[rootNode]
		if !found {
			rootHash := t.RootHash()
			return fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(rootHash[:]))
		}

		encTrie := flattener.EncodeTrie(t, rootIndex, scratch)
		_, err = crc32Writer.Write(encTrie)
		if err != nil {
			return fmt.Errorf("cannot serialize trie: %w", err)
		}
	}

	// all trie nodes have been seralized into the checkpoint file, now
	// write footer with nodes count and tries count.
	footer := scratch[:encNodeCountSize+encTrieCountSize]
	binary.BigEndian.PutUint64(footer, nodeCounter-1) // -1 to account for 0 node meaning nil
	binary.BigEndian.PutUint16(footer[encNodeCountSize:], uint16(len(tries)))

	_, err = crc32Writer.Write(footer)
	if err != nil {
		return fmt.Errorf("cannot write checkpoint footer: %w", err)
	}

	// Write CRC32 sum of the footer for validation
	crc32buf := scratch[:crc32SumSize]
	binary.BigEndian.PutUint32(crc32buf, crc32Writer.Crc32())

	_, err = writer.Write(crc32buf)
	if err != nil {
		return fmt.Errorf("cannot write CRC32: %w", err)
	}

	return nil
}

func logProgress(msg string, estimatedSubtrieNodeCount int, logger zerolog.Logger) func(nodeCounter uint64) {
	lg := util.LogProgress(
		logger,
		util.DefaultLogProgressConfig(
			msg,
			estimatedSubtrieNodeCount,
		),
	)
	return func(index uint64) {
		lg(1)
	}
}

// storeUniqueNodes iterates and serializes unique nodes for trie with given root node.
// It also saves unique nodes and node counter in visitedNodes map.
// It returns nodeCounter and error (if any).
func storeUniqueNodes(
	root *node.Node,
	visitedNodes map[*node.Node]uint64,
	nodeCounter uint64,
	scratch []byte,
	writer io.Writer,
	nodeCounterUpdated func(nodeCounter uint64), // for logging estimated progress
) (uint64, error) {

	for itr := flattener.NewUniqueNodeIterator(root, visitedNodes); itr.Next(); {
		n := itr.Value()

		visitedNodes[n] = nodeCounter
		nodeCounter++
		nodeCounterUpdated(nodeCounter)

		var lchildIndex, rchildIndex uint64

		if lchild := n.LeftChild(); lchild != nil {
			var found bool
			lchildIndex, found = visitedNodes[lchild]
			if !found {
				hash := lchild.Hash()
				return 0, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
			}
		}
		if rchild := n.RightChild(); rchild != nil {
			var found bool
			rchildIndex, found = visitedNodes[rchild]
			if !found {
				hash := rchild.Hash()
				return 0, fmt.Errorf("internal error: missing node with hash %s", hex.EncodeToString(hash[:]))
			}
		}

		encNode := flattener.EncodeNode(n, lchildIndex, rchildIndex, scratch)
		_, err := writer.Write(encNode)
		if err != nil {
			return 0, fmt.Errorf("cannot serialize node: %w", err)
		}
	}

	return nodeCounter, nil
}

// storeUniquePayloadlessNodes iterates and serializes unique payloadless nodes for trie with given root node.
// It also saves unique nodes and node counter in visitedNodes map.
// It returns nodeCounter and error (if any).
func storeUniquePayloadlessNodes(
	root *payloadless.Node,
	visitedNodes map[*payloadless.Node]uint64,
	nodeCounter uint64,
	scratch []byte,
	writer io.Writer,
	nodeCounterUpdated func(nodeCounter uint64), // for logging estimated progress
) (uint64, error) {

	for itr := payloadless.NewUniqueNodeIterator(root, visitedNodes); itr.Next(); {
		n := itr.Value()

		visitedNodes[n] = nodeCounter
		nodeCounter++
		nodeCounterUpdated(nodeCounter)

		var lchildIndex, rchildIndex uint64

		if lchild := n.LeftChild(); lchild != nil {
			var found bool
			lchildIndex, found = visitedNodes[lchild]
			if !found {
				hash := lchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(hash[:]))
			}
		}
		if rchild := n.RightChild(); rchild != nil {
			var found bool
			rchildIndex, found = visitedNodes[rchild]
			if !found {
				hash := rchild.Hash()
				return 0, fmt.Errorf("internal error: missing payloadless node with hash %s", hex.EncodeToString(hash[:]))
			}
		}

		encNode := payloadless.EncodeNode(n, lchildIndex, rchildIndex, scratch)
		_, err := writer.Write(encNode)
		if err != nil {
			return 0, fmt.Errorf("cannot serialize payloadless node: %w", err)
		}
	}

	return nodeCounter, nil
}

// getNodesAtLevel returns 2^level nodes at given level in breadth-first order.
// It guarantees size and order of returned nodes (nil element if no node at the position).
// For example, given nil root and level 3, getNodesAtLevel returns a slice
// of 2^3 nil elements.
func getNodesAtLevel(root *node.Node, level uint) []*node.Node {
	nodes := []*node.Node{root}
	nodesLevel := uint(0)

	// Use breadth first traversal to get all nodes at given level.
	// If a node isn't found, a nil node is used in its place.
	for nodesLevel < level {
		nextLevel := nodesLevel + 1
		nodesAtNextLevel := make([]*node.Node, 1<<nextLevel)

		for i, n := range nodes {
			if n != nil {
				nodesAtNextLevel[i*2] = n.LeftChild()
				nodesAtNextLevel[i*2+1] = n.RightChild()
			}
		}

		nodes = nodesAtNextLevel
		nodesLevel = nextLevel
	}

	return nodes
}

// getPayloadlessNodesAtLevel returns 2^level payloadless nodes at given level in breadth-first order.
// It guarantees size and order of returned nodes (nil element if no node at the position).
// For example, given nil root and level 3, getPayloadlessNodesAtLevel returns a slice
// of 2^3 nil elements.
func getPayloadlessNodesAtLevel(root *payloadless.Node, level uint) []*payloadless.Node {
	nodes := []*payloadless.Node{root}
	nodesLevel := uint(0)

	// Use breadth first traversal to get all nodes at given level.
	// If a node isn't found, a nil node is used in its place.
	for nodesLevel < level {
		nextLevel := nodesLevel + 1
		nodesAtNextLevel := make([]*payloadless.Node, 1<<nextLevel)

		for i, n := range nodes {
			if n != nil {
				nodesAtNextLevel[i*2] = n.LeftChild()
				nodesAtNextLevel[i*2+1] = n.RightChild()
			}
		}

		nodes = nodesAtNextLevel
		nodesLevel = nextLevel
	}

	return nodes
}

// LoadCheckpoint loads a full-mtrie (V6 and earlier) checkpoint by number.
// V7 (payloadless) checkpoints have a different in-memory type and are loaded
// via [OpenAndReadCheckpointV7] instead.
// Deprecated: use LoadCheckpointV6 for explicit loading of V6 checkpoints.
// This function will be removed in the future when V6 checkpoints are no longer supported.
func (c *Checkpointer) LoadCheckpoint(checkpoint int) ([]*trie.MTrie, error) {
	return c.LoadCheckpointV6(checkpoint)
}

// LoadCheckpointV6 loads a V6 checkpoint by number. Returns an error if not found.
func (c *Checkpointer) LoadCheckpointV6(checkpoint int) ([]*trie.MTrie, error) {
	v6Path := path.Join(c.dir, NumberToFilename(checkpoint))
	return LoadCheckpoint(v6Path, c.wal.log)
}

func (c *Checkpointer) LoadRootCheckpoint() ([]*trie.MTrie, error) {
	filepath := path.Join(c.dir, bootstrap.FilenameWALRootCheckpoint)
	return LoadCheckpoint(filepath, c.wal.log)
}

// LoadRootCheckpointV7 loads the V7 (payloadless) root checkpoint as a set of
// payloadless tries. It is the payloadless analog of [Checkpointer.LoadRootCheckpoint].
//
// No error returns are expected during normal operation.
func (c *Checkpointer) LoadRootCheckpointV7() ([]*payloadless.MTrie, error) {
	fileName := bootstrap.FilenameWALRootCheckpoint + V7FileSuffix
	return OpenAndReadCheckpointV7(c.dir, fileName, c.wal.log)
}

// LoadLatestCheckpointV7 loads the most recent usable V7 (payloadless) checkpoint from
// the WAL directory and returns its tries together with the number of the loaded
// checkpoint.
//
// It tries the newest numbered V7 checkpoint first, falling back to older ones if
// a checkpoint file fails to load. This mirrors the V6 checkpoint selection in
// [DiskWAL.replay]. The returned `loadedCheckpoint` is the number of the numbered
// checkpoint that was loaded, used by callers to determine the first WAL segment
// to replay.
//
// When no numbered V7 checkpoint is usable, it falls back to the V7 root
// checkpoint (converted from the V6 root.checkpoint during bootstrap), if present.
// In that case, and when no V7 checkpoint of either kind exists, `loadedCheckpoint`
// is -1, signalling that all segments must be replayed on top of the returned
// tries (which is the empty slice when no checkpoint exists at all).
//
// No error returns are expected during normal operation.
func (c *Checkpointer) LoadLatestCheckpointV7() (tries []*payloadless.MTrie, loadedCheckpoint int, err error) {
	checkpoints, err := c.CheckpointsV7()
	if err != nil {
		return nil, -1, fmt.Errorf("cannot list V7 checkpoints: %w", err)
	}

	// Try the newest V7 checkpoint first, falling back to older ones if a file
	// fails to load. This mirrors the V6 checkpoint selection in [DiskWAL.replay].
	for i := len(checkpoints) - 1; i >= 0; i-- {
		num := checkpoints[i]
		name := NumberToFilenameV7(num)
		tries, err := OpenAndReadCheckpointV7(c.dir, name, c.wal.log)
		if err != nil {
			c.wal.log.Warn().Int("checkpoint", num).Err(err).
				Msg("V7 checkpoint loading failed; falling back to older checkpoint")
			continue
		}
		c.wal.log.Info().Int("checkpoint", num).Int("trie_count", len(tries)).
			Msg("loaded V7 checkpoint")
		return tries, num, nil
	}

	// No numbered V7 checkpoint loaded: fall back to the V7 root checkpoint, if
	// present. This is the payloadless analog of the root-checkpoint branch in
	// [DiskWAL.replay]; like that branch it does not advance the replay start
	// (loadedCheckpoint stays -1), so all segments are replayed on top of the
	// root state.
	hasV7Root, err := c.HasRootCheckpointV7()
	if err != nil {
		return nil, -1, fmt.Errorf("cannot check for V7 root checkpoint: %w", err)
	}
	if hasV7Root {
		tries, err := c.LoadRootCheckpointV7()
		if err != nil {
			return nil, -1, fmt.Errorf("failed to load V7 root checkpoint: %w", err)
		}
		c.wal.log.Info().Int("trie_count", len(tries)).
			Msg("loaded V7 root checkpoint")
		return tries, -1, nil
	}

	return nil, -1, nil
}

func (c *Checkpointer) HasRootCheckpoint() (bool, error) {
	return HasRootCheckpoint(c.dir)
}

// HasRootCheckpointV7 checks if a V7 (payloadless) root checkpoint exists.
func (c *Checkpointer) HasRootCheckpointV7() (bool, error) {
	return HasRootCheckpointV7(c.dir)
}

func HasRootCheckpoint(dir string) (bool, error) {
	if _, err := os.Stat(path.Join(dir, bootstrap.FilenameWALRootCheckpoint)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// HasRootCheckpointV7 checks if a V7 (payloadless) root checkpoint exists.
func HasRootCheckpointV7(dir string) (bool, error) {
	if _, err := os.Stat(path.Join(dir, bootstrap.FilenameWALRootCheckpoint+V7FileSuffix)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (c *Checkpointer) RemoveCheckpoint(checkpoint int) error {
	// Try to remove both V6 and V7 versions if they exist
	v6Name := NumberToFilename(checkpoint)
	v7Name := NumberToFilenameV7(checkpoint)

	v6Err := deleteCheckpointFiles(c.dir, v6Name)
	v7Err := deleteCheckpointFiles(c.dir, v7Name)

	// If both failed, return combined error
	if v6Err != nil && v7Err != nil {
		return fmt.Errorf("failed to remove checkpoint %d: v6 error: %w, v7 error: %v", checkpoint, v6Err, v7Err)
	}
	return nil
}

// RemoveCheckpointV6 deletes only the V6 (full-mtrie) part files for the given
// checkpoint number, leaving any same-numbered V7 file in place. This is used
// by the V6 compactor's retention logic so V7 checkpoints owned by a separate
// writer aren't collaterally damaged.
func (c *Checkpointer) RemoveCheckpointV6(checkpoint int) error {
	v6Name := NumberToFilename(checkpoint)
	if err := deleteCheckpointFiles(c.dir, v6Name); err != nil {
		return fmt.Errorf("failed to remove V6 checkpoint %d: %w", checkpoint, err)
	}
	return nil
}

// RemoveCheckpointV7 deletes only the V7 (payloadless) part files for the given
// checkpoint number, leaving any same-numbered V6 file in place. This is used
// by the payloadless compactor's retention logic so V6 checkpoints owned by a
// separate writer aren't collaterally damaged.
func (c *Checkpointer) RemoveCheckpointV7(checkpoint int) error {
	v7Name := NumberToFilenameV7(checkpoint)
	if err := deleteCheckpointFiles(c.dir, v7Name); err != nil {
		return fmt.Errorf("failed to remove V7 checkpoint %d: %w", checkpoint, err)
	}
	return nil
}

func LoadCheckpoint(filepath string, logger zerolog.Logger) (
	tries []*trie.MTrie,
	errToReturn error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkpoint file %s: %w", filepath, err)
	}
	defer func() {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict file %s from Linux page cache: %s", filepath, evictErr)
			// No need to return this error because it's possible to continue normal operations.
		}

		errToReturn = closeAndMergeError(file, errToReturn)
	}()

	return readCheckpoint(file, logger)
}

func readCheckpoint(f *os.File, logger zerolog.Logger) ([]*trie.MTrie, error) {

	// Read header: magic (2 bytes) + version (2 bytes)
	header := make([]byte, headerSize)
	_, err := io.ReadFull(f, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header: %w", err)
	}

	// Decode header
	magicBytes := binary.BigEndian.Uint16(header)
	version := binary.BigEndian.Uint16(header[encMagicSize:])

	// Reset offset
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	if magicBytes != MagicBytesCheckpointHeader {
		return nil, fmt.Errorf("unknown file format. Magic constant %x does not match expected %x", magicBytes, MagicBytesCheckpointHeader)
	}

	switch version {
	case VersionV1, VersionV3:
		return readCheckpointV3AndEarlier(f, version)
	case VersionV4:
		return readCheckpointV4(f)
	case VersionV5:
		return readCheckpointV5(f, logger)
	case VersionV6:
		return readCheckpointV6(f, logger)
	case VersionV7:
		// V7 (payloadless) returns *payloadless.MTrie rather than *trie.MTrie,
		// so it does not share this dispatcher. Use OpenAndReadCheckpointV7.
		return nil, fmt.Errorf("V7 (payloadless) checkpoints must be loaded via OpenAndReadCheckpointV7")
	default:
		return nil, fmt.Errorf("unsupported file version %x", version)
	}
}

type nodeWithRegMetrics struct {
	n        *node.Node
	regCount uint64
	regSize  uint64
}

// readCheckpointV3AndEarlier deserializes checkpoint file (version 3 and earlier) and returns a list of tries.
// Header (magic and version) is verified by the caller.
// This function is for backwards compatibility, not optimized.
func readCheckpointV3AndEarlier(f *os.File, version uint16) ([]*trie.MTrie, error) {

	var bufReader io.Reader = bufio.NewReaderSize(f, defaultBufioReadSize)
	crcReader := NewCRC32Reader(bufReader)

	var reader io.Reader

	if version != VersionV3 {
		reader = bufReader
	} else {
		reader = crcReader
	}

	// Read header (magic + version), node count, and trie count.
	header := make([]byte, headerSize+encNodeCountSize+encTrieCountSize)

	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, fmt.Errorf("cannot read header: %w", err)
	}

	// Magic and version are verified by the caller.

	// Decode node count and trie count
	nodesCount := binary.BigEndian.Uint64(header[headerSize:])
	triesCount := binary.BigEndian.Uint16(header[headerSize+encNodeCountSize:])

	nodes := make([]nodeWithRegMetrics, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		n, regCount, regSize, err := flattener.ReadNodeFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			if nodeIndex >= uint64(i) {
				return nil, 0, 0, fmt.Errorf("sequence of stored nodes does not satisfy Descendents-First-Relationship")
			}
			nm := nodes[nodeIndex]
			return nm.n, nm.regCount, nm.regSize, nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i].n = n
		nodes[i].regCount = regCount
		nodes[i].regSize = regSize
	}

	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrieFromCheckpointV3AndEarlier(reader, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			if nodeIndex >= uint64(len(nodes)) {
				return nil, 0, 0, fmt.Errorf("sequence of stored nodes doesn't contain node")
			}
			nm := nodes[nodeIndex]
			return nm.n, nm.regCount, nm.regSize, nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read trie %d: %w", i, err)
		}
		tries[i] = trie
	}

	if version == VersionV3 {
		crc32buf := make([]byte, crc32SumSize)

		_, err := io.ReadFull(bufReader, crc32buf)
		if err != nil {
			return nil, fmt.Errorf("cannot read CRC32: %w", err)
		}

		readCrc32 := binary.BigEndian.Uint32(crc32buf)

		calculatedCrc32 := crcReader.Crc32()

		if calculatedCrc32 != readCrc32 {
			return nil, fmt.Errorf("checkpoint checksum failed! File contains %x but calculated crc32 is %x", readCrc32, calculatedCrc32)
		}
	}

	return tries, nil
}

// readCheckpointV4 decodes checkpoint file (version 4) and returns a list of tries.
// Header (magic and version) is verified by the caller.
// This function is for backwards compatibility.
func readCheckpointV4(f *os.File) ([]*trie.MTrie, error) {

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// Read footer to get node count and trie count

	// footer offset: nodes count (8 bytes) + tries count (2 bytes) + CRC32 sum (4 bytes)
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum

	// Seek to footer
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to footer: %w", err)
	}

	footer := scratch[:footerSize]

	_, err = io.ReadFull(f, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	// Decode node count and trie count
	nodesCount := binary.BigEndian.Uint64(footer)
	triesCount := binary.BigEndian.Uint16(footer[encNodeCountSize:])

	// Seek to the start of file
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	var bufReader io.Reader = bufio.NewReaderSize(f, defaultBufioReadSize)
	crcReader := NewCRC32Reader(bufReader)
	var reader io.Reader = crcReader

	// Read header: magic (2 bytes) + version (2 bytes)
	// No action is needed for header because it is verified by the caller.

	_, err = io.ReadFull(reader, scratch[:headerSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read header: %w", err)
	}

	// nodes's element at index 0 is a special, meaning nil .
	nodes := make([]nodeWithRegMetrics, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	for i := uint64(1); i <= nodesCount; i++ {
		n, regCount, regSize, err := flattener.ReadNodeFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			if nodeIndex >= uint64(i) {
				return nil, 0, 0, fmt.Errorf("sequence of stored nodes does not satisfy Descendents-First-Relationship")
			}
			nm := nodes[nodeIndex]
			return nm.n, nm.regCount, nm.regSize, nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i].n = n
		nodes[i].regCount = regCount
		nodes[i].regSize = regSize
	}

	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrieFromCheckpointV4(reader, scratch, func(nodeIndex uint64) (*node.Node, uint64, uint64, error) {
			if nodeIndex >= uint64(len(nodes)) {
				return nil, 0, 0, fmt.Errorf("sequence of stored nodes doesn't contain node")
			}
			nm := nodes[nodeIndex]
			return nm.n, nm.regCount, nm.regSize, nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read trie %d: %w", i, err)
		}
		tries[i] = trie
	}

	// Read footer again for crc32 computation
	// No action is needed.
	_, err = io.ReadFull(reader, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	// Read CRC32
	crc32buf := scratch[:crc32SumSize]
	_, err = io.ReadFull(bufReader, crc32buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read CRC32: %w", err)
	}

	readCrc32 := binary.BigEndian.Uint32(crc32buf)

	calculatedCrc32 := crcReader.Crc32()

	if calculatedCrc32 != readCrc32 {
		return nil, fmt.Errorf("checkpoint checksum failed! File contains %x but calculated crc32 is %x", readCrc32, calculatedCrc32)
	}

	return tries, nil
}

// readCheckpointV5 decodes checkpoint file (version 5) and returns a list of tries.
// Checkpoint file header (magic and version) are verified by the caller.
func readCheckpointV5(f *os.File, logger zerolog.Logger) ([]*trie.MTrie, error) {
	logger.Info().Msgf("reading v5 checkpoint file")

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// Read footer to get node count and trie count

	// footer offset: nodes count (8 bytes) + tries count (2 bytes) + CRC32 sum (4 bytes)
	const footerOffset = encNodeCountSize + encTrieCountSize + crc32SumSize
	const footerSize = encNodeCountSize + encTrieCountSize // footer doesn't include crc32 sum

	// Seek to footer
	_, err := f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to footer: %w", err)
	}

	footer := scratch[:footerSize]

	_, err = io.ReadFull(f, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	// Decode node count and trie count
	nodesCount := binary.BigEndian.Uint64(footer)
	triesCount := binary.BigEndian.Uint16(footer[encNodeCountSize:])

	// Seek to the start of file
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("cannot seek to start of file: %w", err)
	}

	var bufReader io.Reader = bufio.NewReaderSize(f, defaultBufioReadSize)
	crcReader := NewCRC32Reader(bufReader)
	var reader io.Reader = crcReader

	// Read header: magic (2 bytes) + version (2 bytes)
	// No action is needed for header because it is verified by the caller.

	_, err = io.ReadFull(reader, scratch[:headerSize])
	if err != nil {
		return nil, fmt.Errorf("cannot read header: %w", err)
	}

	// nodes's element at index 0 is a special, meaning nil .
	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	logging := logProgress("reading trie nodes", int(nodesCount), logger)

	for i := uint64(1); i <= nodesCount; i++ {
		n, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(i) {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = n
		logging(i)
	}

	logger.Info().Msgf("finished loading %v trie nodes, start loading %v tries", nodesCount, triesCount)

	for i := uint16(0); i < triesCount; i++ {
		trie, err := flattener.ReadTrie(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= uint64(len(nodes)) {
				return nil, fmt.Errorf("sequence of stored nodes doesn't contain node")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read trie %d: %w", i, err)
		}
		tries[i] = trie
	}

	// Read footer again for crc32 computation
	// No action is needed.
	_, err = io.ReadFull(reader, footer)
	if err != nil {
		return nil, fmt.Errorf("cannot read footer: %w", err)
	}

	// Read CRC32
	crc32buf := scratch[:crc32SumSize]
	_, err = io.ReadFull(bufReader, crc32buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read CRC32: %w", err)
	}

	readCrc32 := binary.BigEndian.Uint32(crc32buf)

	calculatedCrc32 := crcReader.Crc32()

	if calculatedCrc32 != readCrc32 {
		return nil, fmt.Errorf("checkpoint checksum failed! File contains %x but calculated crc32 is %x", readCrc32, calculatedCrc32)
	}

	return tries, nil
}

// evictFileFromLinuxPageCache advises Linux to evict a file from Linux page cache.
// A use case is when a new checkpoint is loaded or created, Linux may cache big
// checkpoint files in memory until evictFileFromLinuxPageCache causes them to be
// evicted from the Linux page cache.  Not calling eviceFileFromLinuxPageCache()
// causes two checkpoint files to be cached for each checkpointing, eventually
// caching hundreds of GB.
// CAUTION: no-op when GOOS != linux.
func evictFileFromLinuxPageCache(f *os.File, fsync bool, logger zerolog.Logger) error {
	err := fadviseNoLinuxPageCache(f.Fd(), fsync)
	if err != nil {
		return err
	}

	size := int64(0)
	fstat, err := f.Stat()
	if err == nil {
		size = fstat.Size()
	}

	logger.Info().Str("filename", f.Name()).Int64("size_mb", size/1024/1024).Msg("evicted file from Linux page cache")
	return nil
}

// Copy the checkpoint file including the part files from the given `from` to
// the `to` directory
// it returns the path of all the copied files
// any error returned are exceptions
func CopyCheckpointFile(filename string, from string, to string) (
	[]string,
	error,
) {
	// It's possible that the trie dir does not yet exist. If not this will create the required path
	err := os.MkdirAll(to, 0700)
	if err != nil {
		return nil, err
	}

	// checkpoint V6 produces multiple checkpoint part files that need to be copied over
	pattern := filePathPattern(from, filename)
	matched, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("could not glob checkpoint file with pattern %v: %w", pattern, err)
	}

	newPaths := make([]string, len(matched))
	// copy the root checkpoint concurrently
	var group errgroup.Group

	for i, match := range matched {
		_, partfile := filepath.Split(match)
		newPath := filepath.Join(to, partfile)
		newPaths[i] = newPath

		match := match
		group.Go(func() error {
			err := utilsio.Copy(match, newPath)
			if err != nil {
				return fmt.Errorf("cannot copy file from %v to %v", match, newPath)
			}
			return nil
		})
	}

	err = group.Wait()
	if err != nil {
		return nil, fmt.Errorf("fail to copy checkpoint files: %w", err)
	}

	return newPaths, nil
}

// SoftlinkCheckpointFile creates soft links of the checkpoint file including the part files from the given `from` to
// the `to` directory
func SoftlinkCheckpointFile(filename string, from string, to string) ([]string, error) {

	// It's possible that the trie dir does not yet exist. If not this will create the required path
	err := os.MkdirAll(to, 0700)
	if err != nil {
		return nil, err
	}

	// checkpoint V6 produces multiple checkpoint part files that need to be copied over
	pattern := filePathPattern(from, filename)
	matched, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("could not glob checkpoint file with pattern %v: %w", pattern, err)
	}

	newPaths := make([]string, len(matched))
	for i, match := range matched {
		_, partfile := filepath.Split(match)
		newPath := filepath.Join(to, partfile)
		newPaths[i] = newPath

		// Check if symlink already exists and points to the correct target
		if fi, err := os.Lstat(newPath); err == nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				// Symlink exists, check if it points to the correct target
				target, err := os.Readlink(newPath)
				if err != nil {
					return nil, fmt.Errorf("cannot read existing symlink %v: %w", newPath, err)
				}
				// Calculate expected relative target for comparison
				symlinkDir := filepath.Dir(newPath)
				expectedRelTarget, _ := filepath.Rel(symlinkDir, match)
				if target == expectedRelTarget || target == match {
					// Symlink already exists and points to correct target, skip creation
					continue
				}
				// Symlink exists but points to different target, this is an error
				return nil, fmt.Errorf("symlink %v already exists but points to %v instead of %v", newPath, target, match)
			}
			// Path exists but is not a symlink, this is an error
			return nil, fmt.Errorf("path %v already exists but is not a symlink", newPath)
		}

		// Create symlink with relative path from newPath to match
		// This ensures the symlink works in both host and container contexts
		symlinkDir := filepath.Dir(newPath)
		relTarget, err := filepath.Rel(symlinkDir, match)
		if err != nil {
			// If relative path calculation fails, use match as-is
			relTarget = match
		}
		err = os.Symlink(relTarget, newPath)
		if err != nil {
			return nil, fmt.Errorf("cannot link file from %v to %v: %w", match, newPath, err)
		}
	}

	return newPaths, nil
}
