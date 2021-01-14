// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// init is called first time this package is imported.
// It creates and initializes the channelRoleMap map.
func init() {
	initializeChannelIdMap()
}

// channelRoleMap keeps a map between channels and the list of flow roles involved in them.
var channelRoleMap map[network.Channel]flow.RoleList

// RolesByChannel returns list of flow roles involved in the channel.
func RolesByChannel(channel network.Channel) (flow.RoleList, bool) {
	if clusterChannel, isCluster := IsClusterChannel(channel); isCluster {
		// replaces channelID with the stripped-off channel prefix
		channel = clusterChannel
	}
	roles, ok := channelRoleMap[channel]
	return roles, ok
}

// ChannelIDsByRole returns a list of all channel IDs the role subscribes to.
func ChannelIDsByRole(role flow.Role) network.ChannelList {
	channels := make(network.ChannelList, 0)
	for channel, roles := range channelRoleMap {
		if roles.Contains(role) {
			channels = append(channels, channel)
		}
	}

	return channels
}

// Channels returns all channels that npdes of any role have subscribed to.
func Channels() network.ChannelList {
	channelIDs := make(network.ChannelList, 0)
	for channelID := range channelRoleMap {
		channelIDs = append(channelIDs, channelID)
	}

	return channelIDs
}

// channels
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

// initializeChannelIdMap initializes an instance of channelRoleMap and populates it with the channels and their
// Note: Please update this map, if a new channel is defined or a the roles subscribing to a channel have changed
// corresponding list of roles.
func initializeChannelIdMap() {
	channelRoleMap = make(map[network.Channel]flow.RoleList)

	// Channels for test
	channelRoleMap[TestNetwork] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelRoleMap[TestMetrics] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}

	// Channels for consensus protocols
	channelRoleMap[ConsensusCommittee] = flow.RoleList{flow.RoleConsensus}

	// Channels for protocols actively synchronizing state across nodes
	channelRoleMap[SyncCommittee] = flow.RoleList{flow.RoleConsensus}
	channelRoleMap[SyncExecution] = flow.RoleList{flow.RoleExecution}

	// Channels for actively pushing entities to subscribers
	channelRoleMap[PushTransactions] = flow.RoleList{flow.RoleCollection}
	channelRoleMap[PushGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	channelRoleMap[PushBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelRoleMap[PushReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	channelRoleMap[PushApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	// Channels for actively requesting missing entities
	channelRoleMap[RequestCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	channelRoleMap[RequestChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	channelRoleMap[RequestReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	// Channel aliases to make the code more readable / more robust to errors
	channelRoleMap[ReceiveGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	channelRoleMap[ReceiveBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelRoleMap[ReceiveReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	channelRoleMap[ReceiveApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	channelRoleMap[ProvideCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	channelRoleMap[ProvideChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	channelRoleMap[ProvideReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	channelRoleMap[syncClusterPrefix] = flow.RoleList{flow.RoleCollection}
	channelRoleMap[consensusClusterPrefix] = flow.RoleList{flow.RoleCollection}
}

// IsClusterChannel returns true if channel is cluster-based.
// At the current implementation, only collection nodes are involved in a cluster-based channels.
// If the channel is a cluster-based one, this method also strips off the channel prefix and returns it.
func IsClusterChannel(channel network.Channel) (network.Channel, bool) {
	if strings.HasPrefix(channel.String(), syncClusterPrefix) {
		return syncClusterPrefix, true
	}

	if strings.HasPrefix(channel.String(), consensusClusterPrefix) {
		return consensusClusterPrefix, true
	}

	return "", false
}

// FullyQualifiedChannelName returns the unique channel name made up of channel name string suffixed with root block id
// The root block id is used to prevent cross talks between nodes on different sporks
func FullyQualifiedChannelName(channel network.Channel, rootBlockID string) string {
	// skip root block suffix, if this is a cluster specific channel. A cluster specific channel is inherently
	// unique for each epoch
	if strings.HasPrefix(string(channel), syncClusterPrefix) || strings.HasPrefix(string(channel), consensusClusterPrefix) {
		return string(channel)
	}
	return fmt.Sprintf("%s/%s", string(channel), rootBlockID)
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
