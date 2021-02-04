package marketplace

import (
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
)

type NetworkConfig struct {
	ServiceAccountAddress       *flowsdk.Address
	FlowTokenAddress            *flowsdk.Address
	FungibleTokenAddress        *flowsdk.Address
	NonFungibleTokenAddress     *flowsdk.Address
	ServiceAccountPrivateKeyHex string
	AccessNodeAddresses         []string
}

type SimulatorConfig struct {
	NumberOfAccounts int
	NumberOfMoments  int
	Delay            time.Duration
}
