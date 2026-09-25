package pebble

import (
	"context"
	"fmt"
	"sync"
	"time"

	flowcrypto "github.com/onflow/crypto/hash"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

const (
	// registerDigestQueueSize is the number of registers the scan of a register store can get
	// ahead of the workers hashing them.
	registerDigestQueueSize = 1000

	// registerDigestScratchSize is the initial size of the buffer a worker hashes a register's
	// canonical key and value hash with.
	registerDigestScratchSize = 128
)

// RegisterDigest is an order-independent fingerprint of the registers of a state: the number of
// registers, and a digest over the keys and values of the registers.
//
// The digest is the XOR of one contribution per register, where the contribution of a register is
// the hash of its canonical key followed by the hash of its value:
//
//	H(key.CanonicalForm() || H(value))
//
// XOR is associative and commutative, so the digest does not depend on the order the registers are
// read in, which lets a register store and a checkpoint of the same state be compared without
// sorting either of them.
//
// The digest detects registers that are missing, added or hold a different value. It is a
// fingerprint for detecting corrupted imports, not a state commitment: it is not anchored to the
// state commitment of the state (see [ComputeRegisterRootHash] for that), and a difference in the
// number of registers is only detected by the register count (XOR cannot distinguish an odd from
// an even number of identical contributions), so compare both fields.
type RegisterDigest struct {
	// RegisterCount is the number of registers that were mixed into the digest.
	RegisterCount uint64

	// Digest is the XOR of the contributions of the registers, see [RegisterDigest].
	Digest hash.Hash
}

// add mixes the given register into the digest, using the given buffer as scratch space.
//
// NOT CONCURRENCY SAFE!
func (d *RegisterDigest) add(key ledger.Key, value []byte, scratch []byte) {
	contribution := registerDigestContribution(key, value, scratch)
	for i := range d.Digest {
		d.Digest[i] ^= contribution[i]
	}
	d.RegisterCount++
}

// registerDigestContribution returns the digest contribution of a register: the hash of the
// register's canonical key followed by the hash of its value. The value is hashed first so that the
// hashed input has a fixed-length tail, which keeps the encoding of key and value unambiguous.
//
// No error returns are expected during normal operation.
func registerDigestContribution(key ledger.Key, value []byte, scratch []byte) hash.Hash {
	var valueHash [flowcrypto.HashLenSHA3_256]byte
	flowcrypto.ComputeSHA3_256(&valueHash, value)

	buf := append(scratch[:0], key.CanonicalForm()...)
	buf = append(buf, valueHash[:]...)

	var contribution [flowcrypto.HashLenSHA3_256]byte
	flowcrypto.ComputeSHA3_256(&contribution, buf)
	return hash.Hash(contribution)
}

// Merge combines the other digest into this digest. Digests of disjoint sets of registers can be
// merged into the digest of their union, in any order.
//
// NOT CONCURRENCY SAFE!
func (d *RegisterDigest) Merge(other RegisterDigest) {
	d.RegisterCount += other.RegisterCount
	for i := range d.Digest {
		d.Digest[i] ^= other.Digest[i]
	}
}

// Equal returns true if both digests describe the same registers: the same number of registers with
// the same keys and values.
func (d RegisterDigest) Equal(other RegisterDigest) bool {
	return d.RegisterCount == other.RegisterCount && d.Digest == other.Digest
}

// String returns a human-readable representation of the digest.
func (d RegisterDigest) String() string {
	// note: hashing the slice, since formatting the array with %x would format its String()
	// representation (the hex string) and hex-encode that
	return fmt.Sprintf("register_count=%d digest=%x", d.RegisterCount, d.Digest[:])
}

// ComputeStoreRegisterDigest computes the digest of the registers of the given register store at
// the given height, see [RegisterDigest].
//
// The registers are read with [Registers.ByKeyPrefix], so the computation validates the state at
// the given height of a register store that has heights above it, and registers that were removed
// at or before the given height are not part of the state.
//
// The scan of the register store is sequential (it walks the store's keys), the workers only hash
// the registers. workerCount must be at least 1.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func ComputeStoreRegisterDigest(
	ctx context.Context,
	log zerolog.Logger,
	registers *Registers,
	height uint64,
	workerCount int,
) (RegisterDigest, error) {
	if workerCount < 1 {
		return RegisterDigest{}, fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

	start := time.Now()
	log.Info().Msgf("computing digest of the register store at height %d with %d workers", height, workerCount)

	type registerValue struct {
		key   ledger.Key
		value []byte
	}

	registerValues := make(chan registerValue, registerDigestQueueSize)
	workerDigests := make([]RegisterDigest, workerCount)

	var workers sync.WaitGroup
	for worker := range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			scratch := make([]byte, 0, registerDigestScratchSize)
			for register := range registerValues {
				workerDigests[worker].add(register.key, register.value, scratch)
			}
		}()
	}

	var scanErr error
	for entry, err := range registers.ByKeyPrefix("", height, nil) {
		if err != nil {
			scanErr = fmt.Errorf("could not scan registers: %w", err)
			break
		}
		if err := ctx.Err(); err != nil {
			scanErr = err
			break
		}

		value, err := entry.Value()
		if err != nil {
			scanErr = fmt.Errorf("could not read register value: %w", err)
			break
		}
		if len(value) == 0 {
			// the register was removed at or before the given height, so it is not part of the
			// state at that height
			continue
		}

		select {
		case registerValues <- registerValue{key: convert.RegisterIDToLedgerKey(entry.Cursor()), value: value}:
		case <-ctx.Done():
			scanErr = ctx.Err()
		}
		if scanErr != nil {
			break
		}
	}

	// the workers return once the channel is closed, whether the scan completed or failed
	close(registerValues)
	workers.Wait()

	if scanErr != nil {
		return RegisterDigest{}, scanErr
	}

	digest := RegisterDigest{}
	for _, workerDigest := range workerDigests {
		digest.Merge(workerDigest)
	}

	log.Info().
		Uint64("height", height).
		Str("digest", digest.String()).
		// note: not using Dur() since default units are ms and this duration is long
		Str("duration", fmt.Sprintf("%v", time.Since(start))).
		Msg("register store digest computed")

	return digest, nil
}

// ComputeCheckpointRegisterDigest computes the digest of the registers of the given checkpoint,
// see [RegisterDigest].
//
// The checkpoint's leaf nodes are read in batches and hashed by workerCount workers. The expected
// root hash is checked while reading, so a checkpoint whose root hash is not the expected one is
// rejected. workerCount must be at least 1.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func ComputeCheckpointRegisterDigest(
	ctx context.Context,
	log zerolog.Logger,
	checkpointDir string,
	checkpointFileName string,
	expectedRootHash ledger.RootHash,
	workerCount int,
) (RegisterDigest, error) {
	if workerCount < 1 {
		return RegisterDigest{}, fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

	start := time.Now()
	log.Info().Msgf("computing digest of checkpoint %s with %d workers", checkpointFileName, workerCount)

	leafNodeBatches := make(chan []*wal.LeafNode, registerBootstrapLeafNodeBatchBufferSize)
	workerDigests := make([]RegisterDigest, workerCount)

	g, gCtx := errgroup.WithContext(ctx)
	for worker := range workerCount {
		g.Go(func() error {
			scratch := make([]byte, 0, registerDigestScratchSize)
			for batch := range leafNodeBatches {
				for _, leafNode := range batch {
					key, err := leafNode.Payload.Key()
					if err != nil {
						return fmt.Errorf("could not get key from register payload: %w", err)
					}
					workerDigests[worker].add(key, leafNode.Payload.Value(), scratch)
				}
			}
			return nil
		})
	}

	// The reader pushes the checkpoint's leaf nodes to the channel and closes it once all part
	// files have been read, so it runs in its own goroutine while the workers read from the
	// channel. It gets the workers' context, so that it stops reading once a worker has run into
	// an error, instead of blocking on a channel nobody consumes any more.
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- wal.OpenAndReadLeafNodesFromCheckpointV6Concurrently(
			gCtx, leafNodeBatches, checkpointDir, checkpointFileName, expectedRootHash, workerCount, log)
	}()

	consumeErr := g.Wait()
	readErr := <-readErrCh

	switch {
	case consumeErr != nil:
		// a worker error cancels the reader, whose error is then only the cancellation
		return RegisterDigest{}, consumeErr
	case readErr != nil:
		return RegisterDigest{}, fmt.Errorf("could not read checkpoint file %s: %w", checkpointFileName, readErr)
	}

	digest := RegisterDigest{}
	for _, workerDigest := range workerDigests {
		digest.Merge(workerDigest)
	}

	log.Info().
		Str("checkpoint", checkpointFileName).
		Str("digest", digest.String()).
		// note: not using Dur() since default units are ms and this duration is long
		Str("duration", fmt.Sprintf("%v", time.Since(start))).
		Msg("checkpoint digest computed")

	return digest, nil
}
