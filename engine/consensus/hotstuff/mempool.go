package hotstuff

type Mempool interface {
	NewPayloadHash() []byte
}
