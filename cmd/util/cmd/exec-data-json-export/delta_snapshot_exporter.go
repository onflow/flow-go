package jsonexporter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type dSnapshot struct {
	DeltaJSONStr string   `json:"delta_json_str"`
	Reads        []string `json:"reads"`
}

// ExportDeltaSnapshots exports all the delta snapshots
func ExportDeltaSnapshots(blockID flow.Identifier, dbPath string, outputPath string) error {

	// traverse backward from the given block (parent block) and fetch by blockHash
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	headers := badger.NewHeaders(cacheMetrics, db)

	activeBlockID := blockID
	outputFile := filepath.Join(outputPath, "delta.jsonl")

	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create delta snapshot output file %w", err)
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	for {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			return nil
		}

		var snap []*snapshot.ExecutionSnapshot
		err = db.View(operation.RetrieveExecutionStateInteractions(activeBlockID, &snap))
		if err != nil {
			return fmt.Errorf("could not load delta snapshot: %w", err)
		}

		if len(snap) < 1 {
			// end of snapshots
			return nil
		}
		m, err := json.Marshal(snap[0].UpdatedRegisters())
		if err != nil {
			return fmt.Errorf("could not load delta snapshot: %w", err)
		}

		reads := make([]string, 0)
		for _, r := range snap[0].ReadSet {

			json, err := json.Marshal(r)
			if err != nil {
				return fmt.Errorf("could not create a json obj for a read registerID: %w", err)
			}
			reads = append(reads, string(json))
		}

		data := dSnapshot{
			DeltaJSONStr: string(m),
			Reads:        reads,
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("could not create a json obj for a delta snapshot: %w", err)
		}
		_, err = writer.WriteString(string(jsonData) + "\n")
		if err != nil {
			return fmt.Errorf("could not write delta snapshot json to the file: %w", err)
		}
		writer.Flush()

		activeBlockID = header.ParentID
	}
}
