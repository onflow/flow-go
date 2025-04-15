package flow

type Index struct {
	GuaranteeIDs    []Identifier
	SealIDs         []Identifier
	ReceiptIDs      []Identifier
	ResultIDs       []Identifier
	ProtocolStateID Identifier
}
