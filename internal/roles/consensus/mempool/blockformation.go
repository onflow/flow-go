package mempool

// HoC == Hash Of Collection

type Collection struct {
	Hash []byte // HoC + whatever memory for signature
	// Signatures []Signature // aggregated signature of the collection producers ("Collector Nodes")
}

type BlockPayload interface {
		Collections() []Collection // should return an array of 1000 dummy HoC
}