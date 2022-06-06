package codec

import (
	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"
)

func init() {
	// initialize the authorized roles map the first time this package is imported.
	initializeChannelToMsgCodesMap()
}

type MsgCodeList []MessageCode

// channelToMsgCodes is a mapping of network channels to the cbor codec code's of the messages communicated on them. Each network channel has a list of messages
// that are expected to be communicated on them.
var channelToMsgCodes map[network.Channel]MsgCodeList

// initializeChannelToMsgCodesMap initializes channelToMsgCodes.
func initializeChannelToMsgCodesMap() {
	channelToMsgCodes = make(map[network.Channel]MsgCodeList)

	// consensus
	channelToMsgCodes[channels.ConsensusCommittee] = []MessageCode{CodeBlockProposal, CodeBlockVote}

	// protocol state sync
	channelToMsgCodes[channels.SyncCommittee] = []MessageCode{CodeSyncRequest, CodeSyncResponse, CodeRangeRequest, CodeBatchRequest, CodeBlockResponse}

	// collections, guarantees & transactions
	channelToMsgCodes[channels.PushGuarantees] = []MessageCode{CodeCollectionGuarantee}
	channelToMsgCodes[channels.ReceiveGuarantees] = channelToMsgCodes[channels.PushGuarantees]

	channelToMsgCodes[channels.PushTransactions] = []MessageCode{CodeTransactionBody, CodeTransaction}
	channelToMsgCodes[channels.ReceiveTransactions] = channelToMsgCodes[channels.PushTransactions]

	// core messages for execution & verification
	channelToMsgCodes[channels.PushReceipts] = []MessageCode{CodeExecutionReceipt}
	channelToMsgCodes[channels.ReceiveReceipts] = channelToMsgCodes[channels.PushReceipts]

	channelToMsgCodes[channels.PushApprovals] = []MessageCode{CodeResultApproval}
	channelToMsgCodes[channels.ReceiveApprovals] = channelToMsgCodes[channels.PushApprovals]

	channelToMsgCodes[channels.PushBlocks] = []MessageCode{CodeBlockProposal}
	channelToMsgCodes[channels.ReceiveBlocks] = channelToMsgCodes[channels.PushBlocks]

	// data exchange for execution of blocks
	channelToMsgCodes[channels.ProvideChunks] = []MessageCode{CodeChunkDataRequest, CodeChunkDataResponse}

	// result approvals
	channelToMsgCodes[channels.ProvideApprovalsByChunk] = []MessageCode{CodeApprovalRequest, CodeApprovalResponse}

	// generic entity exchange engines all use EntityRequest and EntityResponse
	channelToMsgCodes[channels.RequestChunks] = []MessageCode{CodeEntityRequest, CodeEntityResponse, CodeChunkDataRequest, CodeChunkDataResponse}
	channelToMsgCodes[channels.RequestCollections] = channelToMsgCodes[channels.RequestChunks]
	channelToMsgCodes[channels.RequestApprovalsByChunk] = channelToMsgCodes[channels.RequestChunks]
	channelToMsgCodes[channels.RequestReceiptsByBlockID] = channelToMsgCodes[channels.RequestChunks]

	// dkg
	channelToMsgCodes[channels.DKGCommittee] = []MessageCode{CodeDKGMessage}

	// cluster based channels
	channelToMsgCodes[channels.SyncClusterPrefix] = []MessageCode{CodeSyncRequest, CodeSyncResponse, CodeRangeRequest, CodeBatchRequest, CodeBlockResponse}
	channelToMsgCodes[channels.ConsensusClusterPrefix] = []MessageCode{CodeClusterBlockProposal, CodeClusterBlockVote, CodeClusterBlockResponse}
}

func MsgCodesByChannel(channel network.Channel) (MsgCodeList, bool) {
	codes, ok := channelToMsgCodes[channel]
	return codes, ok
}

func (m MsgCodeList) Contains(code MessageCode) bool {
	for _, c := range m {
		if c == code {
			return true
		}
	}

	return false
}
