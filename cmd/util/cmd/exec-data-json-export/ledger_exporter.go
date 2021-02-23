package jsonexporter

import (
	"encoding/hex"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/module/metrics"
)

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(ledgerPath string, targetstate string, outputPath string) error {

	led, err := complete.NewLedger(ledgerPath, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, nil, complete.DefaultPathFinderVersion, complete.DefaultHasherVersion)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}
	state, err := hex.DecodeString(targetstate)
	if err != nil {
		return fmt.Errorf("failed to decode hex code of state: %w", err)
	}
	err = led.DumpTrieAsJSON(ledger.State(state), outputPath)
	if err != nil {
		return fmt.Errorf("cannot dump trie as json: %w", err)
	}
	return nil
}
