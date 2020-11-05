package run

import (
	"fmt"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/crypto"
)

func GenerateStakingTransaction(
	privateKeys []crypto.PrivateKey,
	stakingContractAddress *flowsdk.Address,
	stakingAmountPerKey uint64) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
