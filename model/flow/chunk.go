package flow

import (
	"fmt"
	"log"

	"github.com/ipfs/go-cid"
	"github.com/vmihailenco/msgpack/v4"
)

var EmptyEventCollectionID Identifier

func init() {
	// Convert hexadecimal string to a byte slice.
	var err error
	emptyEventCollectionHex := "0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8"
	EmptyEventCollectionID, err = HexStringToIdentifier(emptyEventCollectionHex)
	if err != nil {
		log.Fatalf("Failed to decode hex: %v", err)
	}
}

type ChunkBody struct {
	// TODO(jord): what about service event chunk?
	CollectionIndex uint

	// execution info
	StartState StateCommitment // start state when starting executing this chunk
	// EventCollection is the Merkle root commitment for the events emitted in this chunk.
	// It commits to the order and content of all events. It also commits to the indices of
	// any service events emitted relative to all service events emitted in the overall block.
	// See [EventsMerkleRootHash] for details.
	EventCollection Identifier
	BlockID         Identifier // Block id of the execution result this chunk belongs to

	// Computation consumption info
	TotalComputationUsed uint64 // total amount of computation used by running all txs in this chunk
	NumberOfTransactions uint64 // number of transactions inside the collection
}

type Chunk struct {
	ChunkBody

	Index uint64 // chunk index inside the ER (starts from zero)
	// EndState inferred from next chunk or from the ER
	EndState StateCommitment
}

func NewChunk(
	blockID Identifier,
	collectionIndex int,
	startState StateCommitment,
	numberOfTransactions int,
	eventCollection Identifier,
	endState StateCommitment,
	totalComputationUsed uint64,
) *Chunk {
	return &Chunk{
		ChunkBody: ChunkBody{
			BlockID:              blockID,
			CollectionIndex:      uint(collectionIndex),
			StartState:           startState,
			NumberOfTransactions: uint64(numberOfTransactions),
			EventCollection:      eventCollection,
			TotalComputationUsed: totalComputationUsed,
		},
		Index:    uint64(collectionIndex),
		EndState: endState,
	}
}

// ID returns a unique id for this entity
func (ch *Chunk) ID() Identifier {
	return MakeID(ch.ChunkBody)
}

// Checksum provides a cryptographic commitment for a chunk content
func (ch *Chunk) Checksum() Identifier {
	return MakeID(ch)
}

// ChunkDataPack holds all register touches (any read, or write).
//
// Note that we have to include merkle paths as storage proof for all registers touched (read or written) for
// the _starting_ state of the chunk (i.e. before the chunk computation updates the registers).
// For instance, if an execution state contains three registers: { A: 1, B: 2, C: 3}, and a certain
// chunk has a tx that assigns A = A + B, then its chunk data pack should include the merkle
// paths for { A: 1, B: 2 } as storage proof.
// C is not included because it's neither read or written by the chunk.
// B is included because it's read by the chunk.
// A is included because it's updated by the chunk, and its value 1 is included because it's
// the value before the chunk computation.
// This is necessary for Verification Nodes to (i) check that the read register values are
// consistent with the starting state's root hash and (ii) verify the correctness of the resulting
// state after the chunk computation. `Proof` includes merkle proofs for all touched registers
// during the execution of the chunk.
// Register proofs order must not be correlated to the order of register reads during
// the chunk execution in order to enforce the SPoCK secret high entropy.
type ChunkDataPack struct {
	ChunkID    Identifier      // ID of the chunk this data pack is for
	StartState StateCommitment // commitment for starting state
	Proof      StorageProof    // proof for all registers touched (read or written) during the chunk execution
	Collection *Collection     // collection executed in this chunk

	// ExecutionDataRoot is the root data structure of an execution_data.BlockExecutionData.
	// It contains the necessary information for a verification node to validate that the
	// BlockExecutionData produced is valid.
	ExecutionDataRoot BlockExecutionDataRoot
}

// NewChunkDataPack returns an initialized chunk data pack.
func NewChunkDataPack(
	chunkID Identifier,
	startState StateCommitment,
	proof StorageProof,
	collection *Collection,
	execDataRoot BlockExecutionDataRoot,
) *ChunkDataPack {
	return &ChunkDataPack{
		ChunkID:           chunkID,
		StartState:        startState,
		Proof:             proof,
		Collection:        collection,
		ExecutionDataRoot: execDataRoot,
	}
}

// ID returns the unique identifier for the concrete view, which is the ID of
// the chunk the view is for.
func (c *ChunkDataPack) ID() Identifier {
	return c.ChunkID
}

// Checksum returns the checksum of the chunk data pack.
func (c *ChunkDataPack) Checksum() Identifier {
	return MakeID(c)
}

// TODO: This is the basic version of the list, we need to substitute it with something like Merkle tree at some point
type ChunkList []*Chunk

func (cl ChunkList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(cl)...)
}

func (cl *ChunkList) Insert(ch *Chunk) {
	*cl = append(*cl, ch)
}

func (cl ChunkList) Items() []*Chunk {
	return cl
}

// Empty returns true if the chunk list is empty. Otherwise it returns false.
func (cl ChunkList) Empty() bool {
	return len(cl) == 0
}

func (cl ChunkList) Indices() []uint64 {
	indices := make([]uint64, len(cl))
	for i, chunk := range cl {
		indices[i] = chunk.Index
	}

	return indices
}

// ByChecksum returns an entity from the list by entity fingerprint
func (cl ChunkList) ByChecksum(cs Identifier) (*Chunk, bool) {
	for _, ch := range cl {
		if ch.Checksum() == cs {
			return ch, true
		}
	}
	return nil, false
}

// ByIndex returns an entity from the list by index
// if requested chunk is within range of list, it returns chunk and true
// if requested chunk is out of the range, it returns nil and false
// boolean return value indicates whether requested chunk is within range
func (cl ChunkList) ByIndex(i uint64) (*Chunk, bool) {
	if i >= uint64(len(cl)) {
		// index out of range
		return nil, false
	}
	return cl[i], true
}

// Len returns the number of Chunks in the list. It is also part of the sort
// interface that makes ChunkList sortable
func (cl ChunkList) Len() int {
	return len(cl)
}

// BlockExecutionDataRoot represents the root of a serialized execution_data.BlockExecutionData.
// The hash of the serialized BlockExecutionDataRoot is the ExecutionDataID used within an
// flow.ExecutionResult.
// Context:
//   - The trie updates in BlockExecutionDataRoot contain the _mutated_ registers only, which is
//     helpful for clients to truslessly replicate the state.
//   - In comparison, the chunk data packs contains all the register values at the chunk's starting
//     state that were _touched_ (written and/or read). This is necessary for Verification Nodes to
//     re-run the chunk the computation.
type BlockExecutionDataRoot struct {
	// BlockID is the ID of the block, whose result this execution data is for.
	BlockID Identifier

	// ChunkExecutionDataIDs is a list of the root CIDs for each serialized execution_data.ChunkExecutionData
	// associated with this block.
	ChunkExecutionDataIDs []cid.Cid
}

// MarshalMsgpack implements the msgpack.Marshaler interface
func (b BlockExecutionDataRoot) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(struct {
		BlockID               Identifier
		ChunkExecutionDataIDs []string
	}{
		BlockID:               b.BlockID,
		ChunkExecutionDataIDs: cidsToStrings(b.ChunkExecutionDataIDs),
	})
}

// UnmarshalMsgpack implements the msgpack.Unmarshaler interface
func (b *BlockExecutionDataRoot) UnmarshalMsgpack(data []byte) error {
	var temp struct {
		BlockID               Identifier
		ChunkExecutionDataIDs []string
	}

	if err := msgpack.Unmarshal(data, &temp); err != nil {
		return err
	}

	b.BlockID = temp.BlockID
	cids, err := stringsToCids(temp.ChunkExecutionDataIDs)

	if err != nil {
		return fmt.Errorf("failed to decode chunk execution data ids: %w", err)
	}

	b.ChunkExecutionDataIDs = cids

	return nil
}

// Helper function to convert a slice of cid.Cid to a slice of strings
func cidsToStrings(cids []cid.Cid) []string {
	if cids == nil {
		return nil
	}
	strs := make([]string, len(cids))
	for i, c := range cids {
		strs[i] = c.String()
	}
	return strs
}

// Helper function to convert a slice of strings to a slice of cid.Cid
func stringsToCids(strs []string) ([]cid.Cid, error) {
	if strs == nil {
		return nil, nil
	}
	cids := make([]cid.Cid, len(strs))
	for i, s := range strs {
		c, err := cid.Decode(s)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cid %v: %w", s, err)
		}
		cids[i] = c
	}
	return cids, nil
}
