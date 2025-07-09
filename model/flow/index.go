package flow

type Index struct {
	CollectionIDs   []Identifier
	SealIDs         []Identifier
	ReceiptIDs      []Identifier
	ResultIDs       []Identifier
	ProtocolStateID Identifier
}
