package extract

import (
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/aead/siphash"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	mgr "github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func fromHex(s string) (b []byte) {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return
}

func extractExecutionState(
	dir string,
	targetHash flow.StateCommitment,
	outputDir string,
	log zerolog.Logger,
	migrate bool,
	report bool,
) error {

	diskWal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return fmt.Errorf("cannot create disk WAL: %w", err)
	}
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	var migrations []ledger.Migration
	var reporters []ledger.Reporter
	if migrate {
		migrations = []ledger.Migration{}
	}

	var hashCollisions uint64

	if report {

		var sipHashKey [siphash.KeySize]byte
		copy(sipHashKey[:], fromHex("000102030405060708090a0b0c0d0e0f"))

		reporters = []ledger.Reporter{
			mgr.DataAnalyzer{
				Log: log,
				AnalyzeValue: func(value interpreter.Value) (bool, error) {

					dictionary, ok := value.(*interpreter.DictionaryValue)
					if !ok {
						return true, nil
					}

					keys := dictionary.Keys().Elements()

					hashes := make(map[uint64]struct{}, len(keys))

					for _, key := range keys {
						// TODO: use Cadence Value KeyString?
						keyString := key.String()
						hash := siphash.Sum64([]byte(keyString), &sipHashKey)
						if _, ok := hashes[hash]; ok {
							atomic.AddUint64(&hashCollisions, 1)
						}
						hashes[hash] = struct{}{}
					}

					return true, nil
				},
			},
		}
	}

	newState, err := led.ExportCheckpointAt(
		ledger.State(targetHash),
		migrations,
		reporters,
		complete.DefaultPathFinderVersion,
		outputDir,
		bootstrap.FilenameWALRootCheckpoint,
	)
	if err != nil {
		return fmt.Errorf("cannot generate the output checkpoint: %w", err)
	}

	if migrate {
		log.Info().Msgf(
			"New state commitment for the exported state is: %s (base64: %s)",
			newState.String(),
			newState.Base64(),
		)
	}

	log.Info().Msgf("hash collisions: %d", hashCollisions)

	return nil
}
