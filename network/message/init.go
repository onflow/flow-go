package message

// init is called first time this package is imported.
// It creates and initializes AuthorizationConfigs for each message type.
func init() {
	initializeMessageAuthConfigsMap()
}

// string constants for all message types sent on the network
const (
	BlockProposal        = "BlockProposal"
	BlockVote            = "BlockVote"
	SyncRequest          = "SyncRequest"
	SyncResponse         = "SyncResponse"
	RangeRequest         = "RangeRequest"
	BatchRequest         = "BatchRequest"
	BlockResponse        = "BlockResponse"
	ClusterBlockProposal = "ClusterBlockProposal"
	ClusterBlockVote     = "ClusterBlockVote"
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
