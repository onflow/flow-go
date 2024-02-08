package load

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/integration/benchmark/account"
)

// SimpleLoad is a load that at setup deploys a contract,
// and at load sends a transaction using that contract.
type SimpleLoad struct {
	loadType         LoadType
	contractName     string
	contractTemplate string
	scriptTemplate   string

	contractAddress flowsdk.Address
}

var _ Load = (*SimpleLoad)(nil)

// NewSimpleLoadType creates a new SimpleLoad.
//   - loadType is the type of the load.
//   - contractName is the name of the contract.
//   - contractTemplate is the template of the contract.
//   - scriptTemplate is the template of the script. It should contain a %s placeholder for
//     the contract address.
func NewSimpleLoadType(
	loadType LoadType,
	contractName string,
	contractTemplate string,
	scriptTemplate string,
) *SimpleLoad {
	return &SimpleLoad{
		loadType:         loadType,
		contractName:     contractName,
		contractTemplate: contractTemplate,
		scriptTemplate:   scriptTemplate,
	}
}

func (l *SimpleLoad) Type() LoadType {
	return l.loadType
}

func (l *SimpleLoad) Setup(log zerolog.Logger, lc LoadContext) error {
	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			// this is going to be the contract address
			l.contractAddress = acc.Address

			deploymentTx := flowsdk.NewTransaction().
				SetReferenceBlockID(lc.ReferenceBlockID()).
				SetScript(blueprints.DeployContractTransactionTemplate)

			err := deploymentTx.AddArgument(cadence.String(l.contractName))
			if err != nil {
				return nil, err
			}
			err = deploymentTx.AddArgument(cadence.String(l.contractTemplate))
			if err != nil {
				return nil, err
			}

			return deploymentTx, nil
		},
	)
}

func (l *SimpleLoad) Load(log zerolog.Logger, lc LoadContext) error {
	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			txScript := fmt.Sprintf(l.scriptTemplate, l.contractAddress)
			return flowsdk.NewTransaction().
				SetScript([]byte(txScript)), nil
		},
	)
}
