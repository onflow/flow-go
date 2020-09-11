package exporter

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/rs/zerolog/log"
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
	events := badger.NewEvents(db)
	activeBlockID := blockID
	done := false

	outputFile := filepath.Join(outputPath, "events.jsonl")

	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create transaction output file %w", err)
	}
	defer fi.Close()

	eventWriter := bufio.NewWriter(fi)
	defer eventWriter.Flush()

	for !done {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			done = true
		}

		evs, err := events.ByBlockID(activeBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not fetch events")
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
	return nil
}
