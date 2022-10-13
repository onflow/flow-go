package testutil

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const EventContract = `
access(all) contract EventContract {

	access(all) event TestEvent(value: Int16)

	access(all) fun EmitEvent() {
		emit TestEvent(value: %d)
	}
}
`

func DeployEventContractTransaction(authorizer flow.Address, chain flow.Chain, eventValue int) *flow.TransactionBody {
	contract := fmt.Sprintf(EventContract, eventValue)
	return CreateContractDeploymentTransaction("EventContract", contract, authorizer, chain)
}

func UnauthorizedDeployEventContractTransaction(authorizer flow.Address, chain flow.Chain, eventValue int) *flow.TransactionBody {
	contract := fmt.Sprintf(EventContract, eventValue)
	return CreateUnauthorizedContractDeploymentTransaction("EventContract", contract, authorizer)
}

func UpdateEventContractTransaction(authorizer flow.Address, chain flow.Chain, eventValue int) *flow.TransactionBody {
	contract := fmt.Sprintf(EventContract, eventValue)
	return UpdateContractDeploymentTransaction("EventContract", contract, authorizer, chain)
}

func CreateEmitEventTransaction(contractAccount, signer flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
			import EventContract from 0x%s

			transaction {
				prepare(acc: AuthAccount) {}
				execute {
						EventContract.EmitEvent()
					}
			}`, contractAccount)),
		).
		AddAuthorizer(signer)
}

func IsServiceEvent(event flow.Event, chainID flow.ChainID) bool {
	serviceEvents, _ := systemcontracts.ServiceEventsForChain(chainID)
	for _, serviceEvent := range serviceEvents.All() {
		if serviceEvent.EventType() == event.Type {
			return true
		}
	}
	return false
}
