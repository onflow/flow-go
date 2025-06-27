package initialize

import (
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InitFvmOptions initializes the FVM options based on the chain ID and headers.
// This function is extracted so that it can be reused in multiple places,
// and ensure that the FVM options are consistent across different components.
func InitFvmOptions(
	chainID flow.ChainID,
	headers storage.Headers,
	transactionFeesDisabled bool,
	vmScriptExecutionEnabled bool,
	vmTransactionExecutionEnabled bool,
) []fvm.Option {
	blockFinder := environment.NewBlockFinder(headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(chainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
		fvm.WithVMScriptExecutionEnabled(vmScriptExecutionEnabled),
		fvm.WithVMTransactionExecutionEnabled(vmTransactionExecutionEnabled),
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Mainnet:
		feesEnabled := !transactionFeesDisabled
		vmOpts = append(vmOpts,
			fvm.WithTransactionFeesEnabled(feesEnabled),
		)
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Localnet,
		flow.Benchnet:
		vmOpts = append(vmOpts,
			fvm.WithContractDeploymentRestricted(false),
		)
	}
	return vmOpts
}
