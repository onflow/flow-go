package jsonexporter

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

type blockSummary struct {
	BlockHeight    uint64   `json:"block_height"`
	BlockID        string   `json:"block_id"`
	ParentBlockID  string   `json:"parent_block_id"`
	ParentVoterIDs []string `json:"parent_voter_ids"`
	// ParentVoterSig []string  `json:"parent_voter_sig"`
	ProposerID string `json:"proposer_id"`
	// ProposerSig    string  `json:"proposer_sig"`
	Timestamp     time.Time `json:"timestamp"`
	CollectionIDs []string  `json:"collection_ids"`
	SealedBlocks  []string  `json:"sealed_blocks"`
}

// ExportBlocks exports blocks (note this only export blocks of the main chain and doesn't export forks)
func ExportBlocks(blockID flow.Identifier, dbPath string, outputPath string) error {

	// traverse backward from the given block (parent block) and fetch by blockHash
	db := common.InitStorage(dbPath)
	defer db.Close()

	cacheMetrics := &metrics.NoopCollector{}
	headers := badger.NewHeaders(cacheMetrics, db)
	index := badger.NewIndex(cacheMetrics, db)
	guarantees := badger.NewGuarantees(cacheMetrics, db)
	seals := badger.NewSeals(cacheMetrics, db)
	payloads := badger.NewPayloads(db, index, guarantees, seals)
	blocks := badger.NewBlocks(db, headers, payloads)

	activeBlockID := blockID
	outputFile := filepath.Join(outputPath, "blocks.jsonl")

	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create block output file %w", err)
	}
	defer fi.Close()

	blockWriter := bufio.NewWriter(fi)
	defer blockWriter.Flush()

	for {
		header, err := headers.ByBlockID(activeBlockID)
		if err != nil {
			// no more header is available
			return nil
		}

		block, err := blocks.ByID(activeBlockID)
		if err != nil {
			// log.Fatal().Err(err).Msg("could not load block")
			return nil
		}

		cols := make([]string, 0)
		for _, g := range block.Payload.Guarantees {
			cols = append(cols, hex.EncodeToString(g.CollectionID[:]))
		}

		seals := make([]string, 0)
		for _, s := range block.Payload.Seals {
			seals = append(seals, hex.EncodeToString(s.BlockID[:]))
		}

		pvIDs := make([]string, 0)
		for _, i := range header.ParentVoterIDs {
			pvIDs = append(pvIDs, hex.EncodeToString(i[:]))
		}

		b := blockSummary{
			BlockID:        hex.EncodeToString(activeBlockID[:]),
			BlockHeight:    header.Height,
			ParentBlockID:  hex.EncodeToString(header.ParentID[:]),
			ParentVoterIDs: pvIDs,
			ProposerID:     hex.EncodeToString(header.ProposerID[:]),
			Timestamp:      header.Timestamp,
			CollectionIDs:  cols,
			SealedBlocks:   seals,
		}

		jsonData, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("could not create a json obj for a block: %w", err)
		}
		_, err = blockWriter.WriteString(string(jsonData) + "\n")
		if err != nil {
			return fmt.Errorf("could not write block json to the file: %w", err)
		}
		blockWriter.Flush()

		activeBlockID = header.ParentID
	}
}
