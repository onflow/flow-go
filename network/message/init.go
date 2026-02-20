package message

import (
	"fmt"
	"slices"
)

var (
	// validationExclusionList list of messages that are allowed to have more than 1 configured allowed protocol.
	validationExclusionList = []string{TestMessage}
)

// init is called first time this package is imported.
// It creates and initializes AuthorizationConfigs for each message type.
func init() {
	initializeMessageAuthConfigsMap()
	validateMessageAuthConfigsMap(validationExclusionList)
}

// validateMessageAuthConfigsMap ensures that each message authorization config has exactly one configured AllowedProtocol either pubsub or unicast.
// This is due to the fact that currently there are no messages that are used with both protocols aside from TestMessage.
func validateMessageAuthConfigsMap(excludeList []string) {
	for _, msgAuthConf := range authorizationConfigs {
		if excludeConfig(msgAuthConf.Name, excludeList) {
			continue
		}

		for _, config := range msgAuthConf.Config {
			if len(config.AllowedProtocols) != 1 {
				panic(fmt.Errorf("error: message authorization config for message type %s should have a single allowed protocol found %d: %s", msgAuthConf.Name, len(config.AllowedProtocols), config.AllowedProtocols))
			}
		}
	}
}

func excludeConfig(name string, excludeList []string) bool {
	return slices.Contains(excludeList, name)
}

// string constants for all message types sent on the network
const (
	BlockProposal        = "BlockProposal"
	BlockVote            = "BlockVote"
	TimeoutObject        = "Timeout"
	SyncRequest          = "SyncRequest"
	SyncResponse         = "SyncResponse"
	RangeRequest         = "RangeRequest"
	BatchRequest         = "BatchRequest"
	BlockResponse        = "BlockResponse"
	ClusterBlockProposal = "ClusterBlockProposal"
	ClusterBlockVote     = "ClusterBlockVote"
	ClusterTimeoutObject = "ClusterTimeout"
	ClusterBlockResponse = "ClusterBlockResponse"
	CollectionGuarantee  = "CollectionGuarantee"
	TransactionBody      = "TransactionBody"
	ExecutionReceipt     = "ExecutionReceipt"
	ResultApproval       = "ResultApproval"
	ChunkDataRequest     = "ChunkDataRequest"
	ChunkDataResponse    = "ChunkDataResponse"
	ApprovalRequest      = "ApprovalRequest"
	ApprovalResponse     = "ApprovalResponse"
	EntityRequest        = "EntityRequest"
	EntityResponse       = "EntityResponse"
	TestMessage          = "TestMessage"
	DKGMessage           = "DKGMessage"
)
