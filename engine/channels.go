// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// init is called first time this package is imported.
// It creates and initializes the channel ID map.
func init() {
	initializeChannelIdMap()
}

// channelIdMap keeps a map between channel IDs and list of flow roles involved in that channel ID.
var channelIdMap map[string]flow.RoleList

// RolesByChannelID returns list of flow roles involved in the channelID.
func RolesByChannelID(channelID string) (flow.RoleList, bool) {
	if clusterChannelID, isCluster := IsClusterChannelID(channelID); isCluster {
		// replaces channelID with the stripped-off channel prefix
		channelID = clusterChannelID
	}
	roles, ok := channelIdMap[channelID]
	return roles, ok
}

// ChannelIDsByRole returns a list of all channel IDs the role subscribes to.
func ChannelIDsByRole(role flow.Role) []string {
	channels := make([]string, 0)
	for channelID, roles := range channelIdMap {
		if roles.Contains(role) {
			channels = append(channels, channelID)
		}
	}

	return channels
}

// UniqueChannelIDsByRole returns list of non-cluster channel IDs with unique identifiers accompanied
// with the list of cluster channel IDs, which the role subscribed to.
func UniqueChannelIDsByRole(role flow.Role) []string {
	added := make(map[flow.Identifier]struct{})
	uniques := make([]string, 0)

	// a channel is added to uniques if it is either a
	// cluster channel, or no non-cluster channel with the same id
	// has already been added to uniques.
	all := ChannelIDsByRole(role)
	for _, channel := range all {
		id := channelIdMap[channel].ID()
		if _, ok := added[id]; ok {
			continue
		}

		if _, ok := IsClusterChannelID(channel); !ok {
			added[id] = struct{}{}
		}

		uniques = append(uniques, channel)
	}

	return uniques
}

// ChannelIDs returns all channelIDs nodes of any role have subscribed to.
func ChannelIDs() []string {
	channelIDs := make([]string, 0)
	for channelID := range channelIdMap {
		channelIDs = append(channelIDs, channelID)
	}

	return channelIDs
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

// initializeChannelIdMap initializes an instance of channelIdMap and populates it with the channel IDs and their
// Note: Please update this map, if a new channel is defined or a the roles subscribing to a channel have changed
// corresponding list of roles.
func initializeChannelIdMap() {
	channelIdMap = make(map[string]flow.RoleList)

	// Channels for test
	channelIdMap[TestNetwork] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelIdMap[TestMetrics] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}

	// Channels for consensus protocols
	channelIdMap[ConsensusCommittee] = flow.RoleList{flow.RoleConsensus}

	// Channels for protocols actively synchronizing state across nodes
	channelIdMap[SyncCommittee] = flow.RoleList{flow.RoleConsensus}
	channelIdMap[SyncExecution] = flow.RoleList{flow.RoleExecution}

	// Channels for actively pushing entities to subscribers
	channelIdMap[PushTransactions] = flow.RoleList{flow.RoleCollection}
	channelIdMap[PushGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	channelIdMap[PushBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelIdMap[PushReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	channelIdMap[PushApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	// Channels for actively requesting missing entities
	channelIdMap[RequestCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	channelIdMap[RequestChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	channelIdMap[RequestReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	// Channel aliases to make the code more readable / more robust to errors
	channelIdMap[ReceiveGuarantees] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus}
	channelIdMap[ReceiveBlocks] = flow.RoleList{flow.RoleCollection, flow.RoleConsensus, flow.RoleExecution,
		flow.RoleVerification, flow.RoleAccess}
	channelIdMap[ReceiveReceipts] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification,
		flow.RoleAccess}
	channelIdMap[ReceiveApprovals] = flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	channelIdMap[ProvideCollections] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}
	channelIdMap[ProvideChunks] = flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	channelIdMap[ProvideReceiptsByBlockID] = flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	channelIdMap[syncClusterPrefix] = flow.RoleList{flow.RoleCollection}
	channelIdMap[consensusClusterPrefix] = flow.RoleList{flow.RoleCollection}
}

// IsClusterChannelID returns true if channel ID is a cluster-related channel ID.
// At the current implementation, only collection nodes are involved in a cluster-related channel ID.
// If the channel ID is a cluster-related one, this method also strips off the channel prefix and returns it.
func IsClusterChannelID(channelID string) (string, bool) {
	if strings.HasPrefix(channelID, syncClusterPrefix) {
		return syncClusterPrefix, true
	}

	if strings.HasPrefix(channelID, consensusClusterPrefix) {
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
