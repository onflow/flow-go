package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

	path := filepath.Join(outputPath, hex.EncodeToString(state)+".trie.jsonl")

	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	state, err := ledger.ToState(stateBytes)
	if err != nil {
		return fmt.Errorf("cannot use the input state: %w", err)
	}
	err = led.DumpTrieAsJSON(state, writer)
	if err != nil {
		return fmt.Errorf("cannot dump trie as json: %w", err)
	}
	return nil
}
