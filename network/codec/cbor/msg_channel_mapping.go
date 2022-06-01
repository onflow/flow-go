package cbor

import (
	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"
)

func init() {
	// initialize the authorized roles map the first time this package is imported.
	initializeChannelToMsgCodesMap()
}

// ChannelToMsgCodes is a mapping of network channels to the cbor codec code's of the messages communicated on them. Each network channel has a list of messages
// that are expected to be communicated on them.
var ChannelToMsgCodes map[network.Channel][]uint8

// initializeChannelToMsgCodesMap initializes ChannelToMsgCodes.
func initializeChannelToMsgCodesMap() {
	ChannelToMsgCodes = make(map[network.Channel][]uint8)

	// consensus
	ChannelToMsgCodes[channels.ConsensusCommittee] = []uint8{CodeBlockProposal, CodeBlockVote}

	// protocol state sync
	ChannelToMsgCodes[channels.SyncCommittee] = []uint8{CodeSyncRequest, CodeSyncResponse, CodeRangeRequest, CodeBatchRequest, CodeBlockResponse}

	// collections, guarantees & transactions
	ChannelToMsgCodes[channels.PushGuarantees] = []uint8{CodeCollectionGuarantee}
	ChannelToMsgCodes[channels.ReceiveGuarantees] = ChannelToMsgCodes[channels.PushGuarantees]

	ChannelToMsgCodes[channels.PushTransactions] = []uint8{CodeTransactionBody, CodeTransaction}
	ChannelToMsgCodes[channels.ReceiveTransactions] = ChannelToMsgCodes[channels.PushTransactions]

	// core messages for execution & verification
	ChannelToMsgCodes[channels.PushReceipts] = []uint8{CodeExecutionReceipt}
	ChannelToMsgCodes[channels.ReceiveReceipts] = ChannelToMsgCodes[channels.PushReceipts]

	ChannelToMsgCodes[channels.PushApprovals] = []uint8{CodeResultApproval}
	ChannelToMsgCodes[channels.ReceiveApprovals] = ChannelToMsgCodes[channels.PushApprovals]

	ChannelToMsgCodes[channels.PushBlocks] = []uint8{CodeBlockProposal}
	ChannelToMsgCodes[channels.ReceiveBlocks] = ChannelToMsgCodes[channels.PushBlocks]

	// data exchange for execution of blocks
	ChannelToMsgCodes[channels.ProvideChunks] = []uint8{CodeChunkDataRequest, CodeChunkDataResponse}

	// result approvals
	ChannelToMsgCodes[channels.ProvideApprovalsByChunk] = []uint8{CodeApprovalRequest, CodeApprovalResponse}

	// generic entity exchange engines all use EntityRequest and EntityResponse
	ChannelToMsgCodes[channels.RequestChunks] = []uint8{CodeEntityRequest, CodeEntityResponse, CodeChunkDataRequest, CodeChunkDataResponse}
	ChannelToMsgCodes[channels.RequestCollections] = ChannelToMsgCodes[channels.RequestChunks]
	ChannelToMsgCodes[channels.RequestApprovalsByChunk] = ChannelToMsgCodes[channels.RequestChunks]
	ChannelToMsgCodes[channels.RequestReceiptsByBlockID] = ChannelToMsgCodes[channels.RequestChunks]

	// dkg
	ChannelToMsgCodes[channels.DKGCommittee] = []uint8{CodeDKGMessage}
}
