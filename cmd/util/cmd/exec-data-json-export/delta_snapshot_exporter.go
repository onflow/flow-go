package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type dMapping struct {
	Hash       string `json:"hash"`
	Owner      string `json:"owner"`
	Key        string `json:"key"`
	Controller string `json:"controller"`
}

type dSnapshot struct {
	DeltaJSONStr string     `json:"delta_json_str"`
	DeltaMapping []dMapping `json:"delta_mapping"`
	Reads        []string   `json:"reads"`
	SpockSecret  string     `json:"spock_secret_data"`
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

		var snap []*delta.Snapshot
		err = db.View(operation.RetrieveExecutionStateInteractions(activeBlockID, &snap))
		if err != nil {
			return fmt.Errorf("could not load delta snapshot: %w", err)
		}

		if len(snap) < 1 {
			// end of snapshots
			return nil
		}
		m, err := snap[0].Delta.MarshalJSON()
		if err != nil {
			return fmt.Errorf("could not load delta snapshot: %w", err)
		}

		dm := make([]dMapping, 0)
		for k, v := range snap[0].Delta.ReadMappings {
			dm = append(dm, dMapping{Hash: hex.EncodeToString([]byte(k)),
				Owner:      hex.EncodeToString([]byte(v.Owner)),
				Key:        hex.EncodeToString([]byte(v.Key)),
				Controller: hex.EncodeToString([]byte(v.Controller))})
		}
		for k, v := range snap[0].Delta.WriteMappings {
			dm = append(dm, dMapping{Hash: hex.EncodeToString([]byte(k)),
				Owner:      hex.EncodeToString([]byte(v.Owner)),
				Key:        hex.EncodeToString([]byte(v.Key)),
				Controller: hex.EncodeToString([]byte(v.Controller))})
		}

		reads := make([]string, 0)
		for _, r := range snap[0].Reads {
			reads = append(reads, hex.EncodeToString(r[:]))
		}

		data := dSnapshot{
			DeltaJSONStr: string(m),
			DeltaMapping: dm,
			Reads:        reads,
			SpockSecret:  hex.EncodeToString(snap[0].SpockSecret),
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
