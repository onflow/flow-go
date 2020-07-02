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

	// Channels for actively requesting missing entities related to transactions
	ExchangeTransactions = 100
	ExchangeCollections  = 101
	ExchangeGuarantees   = 102

	// Channels for actively requesting missing entities related to consensus
	ExchangeHeaders  = 110
	ExchangeIndexes  = 111
	ExchangePayloads = 112
	ExchangeBlocks   = 113

	// Channels for actively requesting missing entities related to execution
	ExchangeChunks    = 120
	ExchangeReceipts  = 121
	ExchangeResults   = 122
	ExchangeApprovals = 123
	ExchangeSeals     = 124

	// Channels for actively pushing entities related to transactions
	PushTransactions = 200
	PushGuarantees   = 201
	PushCollections  = 202

	// Channels for actively pushing entities related to blocks
	PushBlocks = 203

	// Channels for actively pushing entities related to execution
	PushChunks    = 204
	PushReceipts  = 205
	PushApprovals = 206
)

func ChannelName(channelID uint8) string {
	return fmt.Sprintf("unknown-channel-%d", channelID)
}

// FullyQualifiedChannelName returns the unique channel name made up of channel name string suffixed with root block id
// The root block id is used to prevent cross talks between nodes on different sporks
func FullyQualifiedChannelName(channelID uint8, rootBlockID string) string {
	return fmt.Sprintf("%s/%s", ChannelName(channelID), rootBlockID)
}
