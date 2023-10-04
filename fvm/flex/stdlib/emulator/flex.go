package emulator

import (
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex/stdlib"
	"github.com/onflow/flow-go/model/flow"
)

// TODO: switch to released version once available
//go:generate env GOPROXY=direct go run github.com/onflow/cadence/runtime/sema/gen@1e04b7af1c098a3deff37931ef33191644606a89 -p emulator ../flex.cdc flex.gen.go

var FlexTypeDefinition = stdlib.FlexTypeDefinition{
	FlexType:                FlexType,
	FlexTypeRunFunctionName: FlexTypeRunFunctionName,
	FlexTypeRunFunctionType: FlexTypeRunFunctionType,
	FlexTypeCreateFlowOwnedAccountFunctionName:   FlexTypeCreateFlowOwnedAccountFunctionName,
	FlexTypeCreateFlowOwnedAccountFunctionType:   FlexTypeCreateFlowOwnedAccountFunctionType,
	Flex_FlexAddressType:                         Flex_FlexAddressType,
	Flex_FlexAddressTypeBytesFieldName:           Flex_FlexAddressTypeBytesFieldName,
	Flex_BalanceType:                             Flex_BalanceType,
	Flex_BalanceTypeFlowFieldName:                Flex_BalanceTypeFlowFieldName,
	Flex_FlowOwnedAccountType:                    Flex_FlowOwnedAccountType,
	Flex_FlowOwnedAccountTypeAddressFunctionName: Flex_FlowOwnedAccountTypeAddressFunctionName,
	Flex_FlowOwnedAccountTypeAddressFunctionType: Flex_FlowOwnedAccountTypeAddressFunctionType,
	Flex_FlowOwnedAccountTypeCallFunctionName:    Flex_FlowOwnedAccountTypeCallFunctionName,
	Flex_FlowOwnedAccountTypeCallFunctionType:    Flex_FlowOwnedAccountTypeCallFunctionType,
}

var FlowToken_VaultType = stdlib.NewFlowTokenVaultType(flow.Emulator.Chain())

var FlexRootAccountAddress = setupFlexRootAccountAddress()

func setupFlexRootAccountAddress() flow.Address {
	flexRootAddress, err := flow.Emulator.Chain().AddressAtIndex(environment.FlexAccountIndex)
	if err != nil {
		panic(err)
	}
	return flexRootAddress
}
