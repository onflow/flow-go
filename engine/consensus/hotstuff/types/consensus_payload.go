package types

type Collection struct{}
type BlockSeal struct{}
type Challenge struct{}
type Adjudication struct{}
type StakeUpdate struct{}

type ConsensusPayload struct {
	Collections   []*Collection
	BlockSeals    []*BlockSeal
	Challenges    []*Challenge
	Adjudications []*Adjudication
	StakeUpdates  []*StakeUpdate
}

//TODO: should be replaced by mempool
func NewConsensusPayload() *ConsensusPayload {
	return &ConsensusPayload{}
}
