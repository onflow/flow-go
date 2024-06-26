package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(ledgerPath string, targetstate string, outputPath string) error {
	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, ledgerPath, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, 0)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), complete.DefaultCacheSize, checkpointDistance, checkpointsToKeep, atomic.NewBool(false), &metrics.NoopCollector{})
	if err != nil {
		return fmt.Errorf("cannot create compactor: %w", err)
	}
	<-compactor.Ready()
	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()
	stateBytes, err := hex.DecodeString(targetstate)
	if err != nil {
		return fmt.Errorf("failed to decode hex code of state: %w", err)
	}
	state, err := ledger.ToState(stateBytes)
	if err != nil {
		return fmt.Errorf("cannot use the input state: %w", err)
	}

	path := filepath.Join(outputPath, state.String()+".trie.jsonl")

	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	err = led.DumpTrieAsJSON(state, writer)
	if err != nil {
		return fmt.Errorf("cannot dump trie as json: %w", err)
	}
	return nil
}
