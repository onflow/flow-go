// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
)

// Enum of channel IDs to avoid accidental conflicts.
const (

	// Channels used for testing
	TestEcho = 00

	// Channels for consensus protocols
	ConsensusCommittee = 10
	ConsensusCluster   = 11

	// Channels for protocols actively synchronizing state across nodes
	SyncCommittee = 20
	SyncCluster   = 21
	SyncExecution = 22

	// Channels for actively pushing entities related to transactions
	PushTransactions = 100
	PushGuarantees   = 101
	PushCollections  = 102

	// Channels for actively pushing entities related to blocks
	PushBlocks = 110

	// Channels for actively pushing entities related to execution
	PushChunks    = 120
	PushReceipts  = 121
	PushApprovals = 122

	// Channels for actively requesting missing entities related to transactions
	ExchangeTransactions = 200
	ExchangeCollections  = 201
	ExchangeGuarantees   = 202

	// Channels for actively requesting missing entities related to consensus
	ExchangeHeaders  = 210
	ExchangeIndexes  = 211
	ExchangePayloads = 212
	ExchangeBlocks   = 213

	// Channels for actively requesting missing entities related to execution
	ExchangeChunks    = 220
	ExchangeReceipts  = 221
	ExchangeResults   = 222
	ExchangeApprovals = 223
	ExchangeSeals     = 224
)

func ChannelName(channelID uint8) string {
	switch channelID {
	case TestEcho:
		return "test-echo"
	case ConsensusCommittee:
		return "consensus-committee"
	case ConsensusCluster:
		return "consensus-cluster"
	case SyncCommittee:
		return "sync-committee"
	case SyncCluster:
		return "sync-cluster"
	case SyncExecution:
		return "sync-execution"
	case PushTransactions:
		return "push-transactions"
	case PushGuarantees:
		return "push-guarantees"
	case PushCollections:
		return "push-collections"
	case PushBlocks:
		return "push-blocks"
	case PushChunks:
		return "push-chunks"
	case PushReceipts:
		return "push-receipts"
	case PushApprovals:
		return "push-approvals"
	case ExchangeTransactions:
		return "exchange-transactions"
	case ExchangeCollections:
		return "exchange-collections"
	case ExchangeGuarantees:
		return "exchange-guarantees"
	case ExchangeHeaders:
		return "exchange-headers"
	case ExchangeIndexes:
		return "exchange-indexes"
	case ExchangePayloads:
		return "exchange-payloads"
	case ExchangeBlocks:
		return "exchange-blocks"
	case ExchangeChunks:
		return "exchange-chunks"
	case ExchangeReceipts:
		return "exchange-receipts"
	case ExchangeResults:
		return "exchange-results"
	case ExchangeApprovals:
		return "exchange-approvals"
	case ExchangeSeals:
		return "exchange-seals"
	}
	return fmt.Sprintf("unknown-channel-%d", channelID)
}

// FullyQualifiedChannelName returns the unique channel name made up of channel name string suffixed with root block id
// The root block id is used to prevent cross talks between nodes on different sporks
func FullyQualifiedChannelName(channelID uint8, rootBlockID string) string {
	return fmt.Sprintf("%s/%s", ChannelName(channelID), rootBlockID)
}
