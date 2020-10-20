// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

// init is a native golang function getting called the first time this package is imported
// externally. It creates and initializes the topic map.
func init() {
	initializeTopicMap()
}

// topicMap keeps a map between topics and list of flow roles involved in that topic.
var topicMap map[string]flow.RoleList

// GetRolesByTopic returns list of flow roles involved in the topic.
func GetRolesByTopic(topic string) (flow.RoleList, bool) {
	roles, ok := topicMap[topic]
	return roles, ok
}

// GetTopicsByRole returns a list of all topics the role subscribes to
func GetTopicsByRole(role flow.Role) []string {
	topics := make([]string, 0)
	for topic, roles := range topicMap {
		if roles.Contains(role) {
			topics = append(topics, topic)
		}
	}

	return topics
}

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

// initializeTopicMap initializes an instance of topicMap and populates it with the topics and their
// corresponding list of roles.
func initializeTopicMap() {
	topicMap = make(map[string]flow.RoleList)

	// Channels for consensus protocols
	topicMap[ConsensusCommittee] = flow.RoleList{flow.RoleConsensus}

	// Channels for protocols actively synchronizing state across nodes
	topicMap[SyncCommittee] = flow.RoleList{flow.RoleConsensus}
	topicMap[SyncExecution] = flow.RoleList{flow.RoleExecution}

	// Channels for actively pushing entities to subscribers
	topicMap[PushTransactions] = flow.RoleList{flow.RoleCollection}
	topicMap[PushGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	topicMap[PushBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	topicMap[PushReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	topicMap[PushApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	// Channels for actively requesting missing entities
	topicMap[RequestCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	topicMap[RequestChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	topicMap[RequestReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	// Channel aliases to make the code more readable / more robust to errors
	topicMap[ReceiveGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	topicMap[ReceiveBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	topicMap[ReceiveReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	topicMap[ReceiveApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	topicMap[ProvideCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	topicMap[ProvideChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	topicMap[ProvideReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	topicMap[syncClusterPrefix] = flow.RoleList{flow.RoleCollection}
	topicMap[consensusClusterPrefix] = flow.RoleList{flow.RoleCollection}
}

// IsClusterTopic returns true if topic is a cluster-related topic.
// At the current implementation, only collection nodes are involved in a cluster-related topic.
func IsClusterTopic(topic string) bool {
	return strings.HasPrefix(topic, syncClusterPrefix) || strings.HasPrefix(topic, consensusClusterPrefix)
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
