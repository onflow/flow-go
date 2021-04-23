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

type result struct {
	ResultID         string   `json:"result_id"`
	PreviousResultID string   `json:"prev_result_id"`
	BlockID          string   `json:"block_id"`
	FinalStateCommit string   `json:"state_commitment"`
	Chunks           []string `json:"chunks"`
}

// ExportResults exports results
func ExportResults(blockID flow.Identifier, dbPath string, outputPath string) error {

	// traverse backward from the given block (parent block) and fetch by blockHash
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	headers := badger.NewHeaders(cacheMetrics, db)
	results := badger.NewExecutionResults(cacheMetrics, db)
	activeBlockID := blockID

	outputFile := filepath.Join(outputPath, "results.jsonl")
	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create exec results output file %w", err)
	}
	defer fi.Close()

	resultWriter := bufio.NewWriter(fi)
	defer resultWriter.Flush()

	for {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			return nil
		}

		res, err := results.ByBlockID(activeBlockID)
		if err != nil {
			return fmt.Errorf("could not fetch events %w", err)
		}

		chunks := make([]string, 0)
		for _, c := range res.Chunks {
			cid := c.ID()
			chunks = append(chunks, hex.EncodeToString(cid[:]))
		}

		resID := res.ID()
		finalState, err := res.FinalStateCommitment()
		if err != nil {
			return fmt.Errorf("export result error: %w", err)
		}
		e := result{
			ResultID:         hex.EncodeToString(resID[:]),
			PreviousResultID: hex.EncodeToString(res.PreviousResultID[:]),
			FinalStateCommit: hex.EncodeToString(finalState[:]),
			BlockID:          hex.EncodeToString(activeBlockID[:]),
			Chunks:           chunks,
		}

		jsonData, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("could not create a json obj for an result: %w", err)
		}
		_, err = resultWriter.WriteString(string(jsonData) + "\n")
		if err != nil {
			return fmt.Errorf("could not write result json to the file: %w", err)
		}
		resultWriter.Flush()

		activeBlockID = header.ParentID
	}
}
