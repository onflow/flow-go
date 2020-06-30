package cmd

import (
	"encoding/hex"
	"time"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootBlock(rootChain string, rootParent string, rootHeight uint64, rootTimestamp string, nodeInfos []model.NodeInfo) *flow.Block {

	chainID := parseChainID(rootChain)
	parentID := parseParentID(rootParent)
	height := rootHeight
	timestamp := parseRootTimestamp(rootTimestamp)
	participants := generateIdentityList(nodeInfos)

	block := run.GenerateRootBlock(chainID, parentID, height, timestamp, participants)

	writeJSON(model.PathRootBlock, block)

	return block
}

func generateIdentityList(nodes []model.NodeInfo) flow.IdentityList {

	list := make([]*flow.Identity, 0, len(nodes))

	for _, node := range nodes {
		ident := node.Identity()
		list = append(list, ident)
	}

	return list
}

func parseChainID(chainID string) flow.ChainID {
	switch chainID {
	case "main":
		return flow.Mainnet
	case "test":
		return flow.Testnet
	case "emulator":
		return flow.Emulator
	default:
		log.Fatal().Str("chain_id", chainID).Msg("invalid chain ID")
		return ""
	}
}

func parseParentID(parentID string) flow.Identifier {
	decoded, err := hex.DecodeString(parentID)
	if err != nil {
		log.Fatal().Str("parent_id", parentID).Msg("invalid parent ID")
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
