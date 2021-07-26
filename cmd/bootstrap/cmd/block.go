package cmd

import (
	"encoding/hex"
	"time"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootBlock(rootChain string, rootParent string, rootHeight uint64, rootTimestamp string) *flow.Block {

	chainID := parseChainID(rootChain)
	parentID := parseParentID(rootParent)
	height := rootHeight
	timestamp := parseRootTimestamp(rootTimestamp)

	block := run.GenerateRootBlock(chainID, parentID, height, timestamp)

	return block
}

func parseChainID(chainID string) flow.ChainID {
	switch chainID {
	case "main":
		return flow.Mainnet
	case "test":
		return flow.Testnet
	case "canary":
		return flow.Canary
	case "bench":
		return flow.Benchnet
	case "local":
		return flow.Localnet
	default:
		log.Fatal().Str("chain_id", chainID).Msg("invalid chain ID")
		return ""
	}
}

func parseParentID(parentID string) flow.Identifier {
	decoded, err := hex.DecodeString(parentID)
	if err != nil {
		log.Fatal().Err(err).Str("parent_id", parentID).Msg("invalid parent ID")
	}
	var id flow.Identifier
	if len(decoded) != len(id) {
		log.Fatal().Str("parent_id", parentID).Msg("invalid parent ID length")
	}
	copy(id[:], decoded[:])
	return id
}

func parseRootTimestamp(timestamp string) time.Time {

	if timestamp == "" {
		return time.Now().UTC()
	}

	rootTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		log.Fatal().Str("root_time", timestamp).Msg("invalid root timestamp")
	}

	return rootTime
}
