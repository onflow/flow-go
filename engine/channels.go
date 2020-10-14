// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

var TopicMap map[string]flow.RoleList

// channel IDs
const (

	// Channels used for testing
	TestNetwork = "test-network"
	TestMetrics = "test-metrics"

	// Channels for consensus protocols
	ConsensusCommittee     = "consensus-committee"
	consensusClusterPrefix = "consensus-cluster" // dynamic channel, use ChannelConsensusCluster function

	// Channels for protocols actively synchronizing state across nodes
	SyncCommittee     = "sync-committee"
	syncClusterPrefix = "sync-cluster" // dynamic channel, use ChannelSyncCluster function
	SyncExecution     = "sync-execution"

	// Channels for actively pushing entities to subscribers
	PushTransactions = "push-transactions"
	PushGuarantees   = "push-guarantees"
	PushBlocks       = "push-blocks"
	PushReceipts     = "push-receipts"
	PushApprovals    = "push-approvals"

	// Channels for actively requesting missing entities
	RequestCollections       = "request-collections"
	RequestChunks            = "request-chunks"
	RequestReceiptsByBlockID = "request-receipts-by-block-id"

	// Channel aliases to make the code more readable / more robust to errors
	ReceiveTransactions = PushTransactions
	ReceiveGuarantees   = PushGuarantees
	ReceiveBlocks       = PushBlocks
	ReceiveReceipts     = PushReceipts
	ReceiveApprovals    = PushApprovals

	ProvideCollections       = RequestCollections
	ProvideChunks            = RequestChunks
	ProvideReceiptsByBlockID = RequestReceiptsByBlockID
)

func Init() {

}

func initializeTopicMap() {
	TopicMap := make(map[string]flow.RoleList)
	// Channels for consensus protocols
	TopicMap[ConsensusCommittee] = flow.RoleList{flow.RoleConsensus}

	// Channels for protocols actively synchronizing state across nodes
	TopicMap[SyncCommittee] = flow.RoleList{flow.RoleConsensus}
	TopicMap[SyncExecution] = flow.RoleList{flow.RoleExecution}

	// Channels for actively pushing entities to subscribers
	TopicMap[PushTransactions] = flow.RoleList{flow.RoleCollection}
	TopicMap[PushGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	TopicMap[PushBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	TopicMap[PushReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleAccess}
	TopicMap[PushApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	// Channels for actively requesting missing entities
	TopicMap[RequestCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	TopicMap[RequestChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	TopicMap[RequestReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	// Channel aliases to make the code more readable / more robust to errors
	TopicMap[ReceiveGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	TopicMap[ReceiveBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	TopicMap[ReceiveReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleAccess}
	TopicMap[ReceiveApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	TopicMap[ProvideCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	TopicMap[ProvideChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	TopicMap[ProvideReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

}

// FullyQualifiedChannelName returns the unique channel name made up of channel name string suffixed with root block id
// The root block id is used to prevent cross talks between nodes on different sporks
func FullyQualifiedChannelName(channelID string, rootBlockID string) string {
	// skip root block suffix, if this is a cluster specific channel. A cluster specific channel is inherently
	// unique for each epoch
	if strings.HasPrefix(channelID, syncClusterPrefix) || strings.HasPrefix(channelID, consensusClusterPrefix) {
		return channelID
	}
	return fmt.Sprintf("%s/%s", channelID, rootBlockID)
}

// ChannelConsensusCluster returns a dynamic cluster consensus channel based on
// the chain ID of the cluster in question.
func ChannelConsensusCluster(clusterID flow.ChainID) string {
	return fmt.Sprintf("%s-%s", consensusClusterPrefix, clusterID)
}

// ChannelSyncCluster returns a dynamic cluster sync channel based on the chain
// ID of the cluster in question.
func ChannelSyncCluster(clusterID flow.ChainID) string {
	return fmt.Sprintf("%s-%s", syncClusterPrefix, clusterID)
}
