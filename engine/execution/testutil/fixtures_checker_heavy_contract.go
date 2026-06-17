package testutil

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

func DeployLocalReplayLimitedTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBodyBuilder {

	var builder strings.Builder
	builder.WriteString("let t = T")
	for range 30 {
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

func DeployGlobalReplayLimitedTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBodyBuilder {

	var builder strings.Builder
	for range 2 {
		builder.WriteString(";let t = T")
		for range 16 {
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
