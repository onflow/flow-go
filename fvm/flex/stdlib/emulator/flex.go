package emulator

import (
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex/stdlib"
	"github.com/onflow/flow-go/model/flow"
)

// TODO: switch to released version once available
//go:generate env GOPROXY=direct go run github.com/onflow/cadence/runtime/sema/gen@1e04b7af1c098a3deff37931ef33191644606a89 -p emulator ../flex.cdc flex.gen.go

var FlexTypeDefinition = stdlib.FlexTypeDefinition{
	FlexType:                           FlexType,
	FlexTypeRunFunctionType:            FlexTypeRunFunctionType,
	FlexTypeRunFunctionName:            FlexTypeRunFunctionName,
	Flex_FlexAddressType:               Flex_FlexAddressType,
	Flex_FlexAddressTypeBytesFieldName: Flex_FlexAddressTypeBytesFieldName,
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
