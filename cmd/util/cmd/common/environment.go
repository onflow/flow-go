package common

import (
	"fmt"

	"github.com/onflow/flow-core-contracts/lib/go/templates"
)

func EnvFromNetwork(network string) (templates.Environment, error) {
	if network == "mainnet" {
		return templates.Environment{
			// https://docs.onflow.org/protocol/core-contracts/flow-id-table-staking/
			IDTableAddress:       "8624b52f9ddcd04a",
			FungibleTokenAddress: "f233dcee88fe0abe",
			FlowTokenAddress:     "1654653399040a61",
			LockedTokensAddress:  "8d0e87b65159ae63",
			StakingProxyAddress:  "62430cf28c26d095",
		}, nil
	}

	if network == "testnet" {
		return templates.Environment{
			IDTableAddress:       "9eca2b38b18b5dfe",
			FungibleTokenAddress: "9a0766d93b6608b7",
			FlowTokenAddress:     "7e60df042a9c0868",
			LockedTokensAddress:  "95e019a17d0e23d7",
			StakingProxyAddress:  "7aad92e5a0715d21",
		}, nil
	}

	if network == "localnet" {
		return templates.Environment{
			IDTableAddress:       "f8d6e0586b0a20c7",
			FungibleTokenAddress: "ee82856bf20e2aa6",
			FlowTokenAddress:     "0ae53cb6e3f42a79",
			LockedTokensAddress:  "f8d6e0586b0a20c7",
			StakingProxyAddress:  "f8d6e0586b0a20c7",
		}, nil
	}

	return templates.Environment{}, fmt.Errorf("invalid network string expecting one of ( mainnet | testnet | localnet )")
}
