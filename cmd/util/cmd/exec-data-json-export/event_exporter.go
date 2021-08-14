package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

type event struct {
	TxID        string `json:"tx_id"`
	TxIndex     uint32 `json:"tx_index"`
	EventIndex  uint32 `json:"event_index"`
	EventType   string `json:"event_type"`
	PayloadHex  string `json:"payload_hex"`
	BlockID     string `json:"block_id"`
	BlockHeight uint64 `json:"block_height"`
}

// ExportEvents exports events
func ExportEvents(blockID flow.Identifier, dbPath string, outputPath string) error {

	// traverse backward from the given block (parent block) and fetch by blockHash
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	headers := badger.NewHeaders(cacheMetrics, db)
	events := badger.NewEvents(cacheMetrics, db)
	activeBlockID := blockID

	outputFile := filepath.Join(outputPath, "events.jsonl")
	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create event output file %w", err)
	}
	defer fi.Close()

	eventWriter := bufio.NewWriter(fi)
	defer eventWriter.Flush()

	for {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			return nil
		}

		evs, err := events.ByBlockID(activeBlockID)
		if err != nil {
			return fmt.Errorf("could not fetch events %w", err)
		}

		for _, ev := range evs {
			e := event{
				TxID:        hex.EncodeToString(ev.TransactionID[:]),
				TxIndex:     ev.TransactionIndex,
				EventIndex:  ev.EventIndex,
				EventType:   string(ev.Type),
				PayloadHex:  hex.EncodeToString(ev.Payload),
				BlockID:     hex.EncodeToString(activeBlockID[:]),
				BlockHeight: header.Height,
			}
			jsonData, err := json.Marshal(e)
			if err != nil {
				return fmt.Errorf("could not create a json obj for an event: %w", err)
			}
			_, err = eventWriter.WriteString(string(jsonData) + "\n")
			if err != nil {
				return fmt.Errorf("could not write event json to the file: %w", err)
			}
			eventWriter.Flush()
		}
		activeBlockID = header.ParentID
	}
}
