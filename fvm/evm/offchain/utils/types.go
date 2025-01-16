package utils

type Event struct {
	FlowBlockHeight uint64 `json:"flow_height"`
	EventType       string `json:"type"`
	EventPayload    string `json:"payload"`
	txIndex         int
	eventIndex      int
}

type Config struct {
	host               string
	evmContractAddress string // no prefix
	startHeight        uint64
	endHeight          uint64
	batchSize          uint64
}

var Devnet51Config = Config{
	host:               "access-001.devnet51.nodes.onflow.org:9000",
	evmContractAddress: "8c5303eaa26202d6",
	startHeight:        uint64(211176670),
	endHeight:          uint64(218215349),
	batchSize:          uint64(50),
}

var Mainnet25Config = Config{
	host:               "access-001.mainnet25.nodes.onflow.org:9000",
	evmContractAddress: "e467b9dd11fa00df",
	startHeight:        uint64(85981135),
	endHeight:          uint64(88226266),
	batchSize:          uint64(50),
}
