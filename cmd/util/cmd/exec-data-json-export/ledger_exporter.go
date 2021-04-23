package jsonexporter

import (
	"encoding/hex"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(ledgerPath string, targetstate string, outputPath string) error {

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, ledgerPath, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		<-diskWal.Done()
	}()
	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, 0)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}
	stateBytes, err := hex.DecodeString(targetstate)
	if err != nil {
		return fmt.Errorf("failed to decode hex code of state: %w", err)
	}

	state, err := ledger.ToState(stateBytes)
	if err != nil {
		return fmt.Errorf("cannot use the input state: %w", err)
	}
	err = led.DumpTrieAsJSON(state, outputPath)
	if err != nil {
		return fmt.Errorf("cannot dump trie as json: %w", err)
	}
	return nil
}
