package blueprints

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

const ContractDeploymentAuthorizedAddressesPathDomain = "storage"
const ContractDeploymentAuthorizedAddressesPathIdentifier = "authorizedAddressesToDeployContracts"

const setContractDeploymentAuthorizersTransactionTemplate = `
transaction {
		prepare(signer: AuthAccount) {
			signer.save([%s], to: /%s/%s)
		}
}
`

// SetContractDeploymentAuthorizersTransaction returns a transaction for setting storage path ...
func SetContractDeploymentAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) *flow.TransactionBody {
	addStrs := make([]string, 0)
	for _, a := range authorized {
		addStrs = append(addStrs, a.HexWithPrefix()+" as Address")
	}

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(setContractDeploymentAuthorizersTransactionTemplate,
			strings.Join(addStrs, ", "),
			ContractDeploymentAuthorizedAddressesPathDomain,
			ContractDeploymentAuthorizedAddressesPathIdentifier))).
		AddAuthorizer(serviceAccount)
}
