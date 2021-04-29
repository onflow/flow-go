package blueprints

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const flowTokenBalanceScriptTemplate = `
import FlowServiceAccount from 0x%s

pub fun main(): UFix64 {
  let acct = getAccount(0x%s)
  return FlowServiceAccount.defaultTokenBalance(acct)
}
`

func FlowTokenBalanceScript(accountAddress, serviceAddress flow.Address) []byte {
	return []byte(fmt.Sprintf(flowTokenBalanceScriptTemplate, serviceAddress, accountAddress))
}

const flowTokenAvailableBalanceScriptTemplate = `
import FlowStorageFees from 0x%s

pub fun main(): UFix64 {
  return FlowStorageFees.defaultTokenAvailableBalance(0x%s)
}
`

func FlowTokenAvailableBalanceScript(accountAddress, serviceAddress flow.Address) []byte {
	return []byte(fmt.Sprintf(flowTokenAvailableBalanceScriptTemplate, serviceAddress, accountAddress))
}

const storageCapacityScriptTemplate = `
import FlowStorageFees from 0x%s

pub fun main(): UFix64 {
	return FlowStorageFees.calculateAccountCapacity(0x%s)
}
`

func StorageCapacityScript(accountAddress, serviceAddress flow.Address) []byte {
	return []byte(fmt.Sprintf(storageCapacityScriptTemplate, serviceAddress, accountAddress))
}
