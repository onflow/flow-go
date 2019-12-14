package def

type Node interface {
	ReceiveFinalisedBlock(*Block)
}
