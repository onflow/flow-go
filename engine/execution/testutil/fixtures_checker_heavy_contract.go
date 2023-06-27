package testutil

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

func DeployLocalReplayLimitedTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {

	var builder strings.Builder
	builder.WriteString("let t = T")
	for i := 0; i < 30; i++ {
		builder.WriteString("<T")
	}
	builder.WriteString(">()")

	return CreateContractDeploymentTransaction(
		"LocalReplayLimited",
		builder.String(),
		authorizer,
		chain,
	)
}

func DeployGlobalReplayLimitedTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {

	var builder strings.Builder
	for j := 0; j < 2; j++ {
		builder.WriteString(";let t = T")
		for i := 0; i < 16; i++ {
			builder.WriteString("<T")
		}
	}

	return CreateContractDeploymentTransaction(
		"GlobalReplayLimited",
		builder.String(),
		authorizer,
		chain,
	)
}
