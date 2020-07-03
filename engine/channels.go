// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
)

// Enum of channel IDs to avoid accidental conflicts.
const (

	// Channels used for testing
	TestNetwork = 00
	TestMetrics = 01

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
	RequestTransactions = 200
	RequestCollections  = 201
	RequestGuarantees   = 202

	// Channels for actively requesting missing entities related to consensus
	RequestHeaders  = 210
	RequestIndexes  = 211
	RequestPayloads = 212
	RequestBlocks   = 213

	// Channels for actively requesting missing entities related to execution
	RequestChunks    = 220
	RequestReceipts  = 221
	RequestResults   = 222
	RequestApprovals = 223
	RequestSeals     = 224

	// Channel aliases to make the code more readable / more robust to errors
	ReceiveTransactions = PushTransactions
	ReceiveGuarantees   = PushGuarantees
	ReceiveCollections  = PushCollections
	ReceiveBlocks       = PushBlocks
	ReceiveChunks       = PushChunks
	ReceiveReceipts     = PushReceipts
	ReceiveApprovals    = PushApprovals
	ProvideTransactinos = RequestTransactions
	ProvideCollections  = RequestCollections
	ProvideGuarantees   = RequestGuarantees
	ProvideHeaders      = RequestHeaders
	ProvideIndexes      = RequestIndexes
	ProvidePayloads     = RequestPayloads
	ProvideBlocks       = RequestBlocks
	ProvideChunks       = RequestChunks
	ProvideReceipts     = RequestReceipts
	ProvideResults      = RequestResults
	ProvideApprovals    = RequestApprovals
	ProvideSeals        = RequestSeals
)

func ChannelName(channelID uint8) string {
	switch channelID {
	case TestNetwork:
		return "test-network"
	case TestMetrics:
		return "test-metrics"
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
	case RequestTransactions:
		return "request-transactions"
	case RequestCollections:
		return "request-collections"
	case RequestGuarantees:
		return "request-guarantees"
	case RequestHeaders:
		return "request-headers"
	case RequestIndexes:
		return "request-indexes"
	case RequestPayloads:
		return "request-payloads"
	case RequestBlocks:
		return "request-blocks"
	case RequestChunks:
		return "request-chunks"
	case RequestReceipts:
		return "request-receipts"
	case RequestResults:
		return "request-results"
	case RequestApprovals:
		return "request-approvals"
	case RequestSeals:
		return "request-seals"
	}
	return fmt.Sprintf("unknown-channel-%d", channelID)
}

// FullyQualifiedChannelName returns the unique channel name made up of channel name string suffixed with root block id
// The root block id is used to prevent cross talks between nodes on different sporks
func FullyQualifiedChannelName(channelID uint8, rootBlockID string) string {
	return fmt.Sprintf("%s/%s", ChannelName(channelID), rootBlockID)
}
