package utils

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
)

const tokenTransferTransactionTemplate = `
import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xTOKENADDRESS

transaction(amount: UFix64, to: Address) {
    let sentVault: @FungibleToken.Vault

    prepare(signer: AuthAccount) {
        let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
			?? panic("Could not borrow reference to the owner's Vault!")
        self.sentVault <- vaultRef.withdraw(amount: amount)
    }

    execute {
        let receiverRef =  getAccount(to)
            .getCapability(/public/flowTokenReceiver)
            .borrow<&{FungibleToken.Receiver}>()
			?? panic("Could not borrow receiver reference to the recipient's Vault")
        receiverRef.deposit(from: <-self.sentVault)
    }
}
`

// TokenTransferTransaction returns a transaction script for transferring `amount` flow tokens to `toAddr` address
func TokenTransferTransaction(ftAddr, flowToken, toAddr *flowsdk.Address, amount float64) (*flowsdk.Transaction, error) {

	withFTAddr := strings.Replace(tokenTransferTransactionTemplate, "0xFUNGIBLETOKENADDRESS", "0x"+ftAddr.Hex(), 1)
	withFlowTokenAddr := strings.Replace(withFTAddr, "0xTOKENADDRESS", "0x"+flowToken.Hex(), 1)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(withFlowTokenAddr))

	cadAmount, err := cadence.NewUFix64(fmt.Sprintf("%f", amount))
	if err != nil {
		return nil, err
	}

	err = tx.AddArgument(cadAmount)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(cadence.BytesToAddress(toAddr.Bytes()))
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// AddKeyToAccountScript returns a transaction script to add keys to an account
func AddKeyToAccountScript() ([]byte, error) {
	return []byte(`
    transaction(keys: [[UInt8]]) {
      prepare(signer: AuthAccount) {
      for key in keys {
        signer.addPublicKey(key)
      }
      }
    }
    `), nil
}

const createAccountsScriptTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(publicKey: [UInt8], count: Int, initialTokenAmount: UFix64) {
  prepare(signer: AuthAccount) {
	let vault = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
      ?? panic("Could not borrow reference to the owner's Vault")

    var i = 0
    while i < count {
      let account = AuthAccount(payer: signer)
      account.addPublicKey(publicKey)

	  let receiver = account.getCapability(/public/flowTokenReceiver)
        .borrow<&{FungibleToken.Receiver}>()
		?? panic("Could not borrow receiver reference to the recipient's Vault")

      receiver.deposit(from: <-vault.withdraw(amount: initialTokenAmount))

      i = i + 1
    }
  }
}
`

// CreateAccountsScript returns a transaction script for creating an account
func CreateAccountsScript(fungibleToken, flowToken flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(createAccountsScriptTemplate, fungibleToken, flowToken))
}

const myFavContract = `
access(all) contract MyFavContract {

    init() {
        self.itemCounter = UInt32(0)
        self.items = []
    }

    // items
    access(all) event NewItemAddedEvent(id: UInt32, metadata: {String: String})

    access(self) var itemCounter: UInt32

    access(all) struct Item {

            pub let itemID: UInt32

            pub let metadata: {String: String}

            init(_ metadata: {String: String}) {
                self.itemID = MyFavContract.itemCounter
                self.metadata = metadata

                // inc the counter
                MyFavContract.itemCounter = MyFavContract.itemCounter + UInt32(1)

                // emit event
                emit NewItemAddedEvent(id: self.itemID, metadata: self.metadata)
            }
    }

    access(self) var items: [Item]

    access(all) fun AddItem(_ metadata: {String: String}){
        let item = Item(metadata)
        self.items.append(item)
    }

    access(all) fun AddManyRandomItems(_ n: Int){
        var i = 0
        while i < n {
            MyFavContract.AddItem({"data": "ABCDEFGHIJKLMNOP"})
            i = i + 1
        }
    }

    // heavy operations
    // computation heavy function
    access(all) fun ComputationHeavy() {
    	var s: Int256 = 1024102410241024
        var i = 0
        var a = Int256(7)
        var b = Int256(5)
        var c = Int256(2)
        while i < 15000 {
            s = s * a
            s = s / b
            s = s / c
            i = i + 1
        }
        log(i)
    }

    access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})

    // event heavy function
    access(all) fun EventHeavy() {
        var s: Int256 = 1024102410241024
        var i = 0

        while i < 220 {
            emit LargeEvent(value: s, str: s.toString(), list:[], dic:{s.toString():s.toString()})
            i = i + 1
        }
        log(i)
    }

    access(all) fun LedgerInteractionHeavy() {
        MyFavContract.AddManyRandomItems(800)
    }
}
`

const deployingMyFavContractScriptTemplate = `
transaction {
  prepare(signer: AuthAccount) {
		signer.contracts.add(name: "%s", code: "%s".decodeHex())
  }
}
`

func DeployingMyFavContractScript() []byte {
	return []byte(fmt.Sprintf(deployingMyFavContractScriptTemplate, "MyFavContract", hex.EncodeToString([]byte(myFavContract))))

}

const eventHeavyScriptTemplate = `
import MyFavContract from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    MyFavContract.EventHeavy()
  }
}
`

func EventHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(eventHeavyScriptTemplate, favContractAddress))
}

const compHeavyScriptTemplate = `
import MyFavContract from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    MyFavContract.ComputationHeavy()
  }
}
`

func ComputationHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(compHeavyScriptTemplate, favContractAddress))
}

const ledgerHeavyScriptTemplate = `
import MyFavContract from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    MyFavContract.LedgerInteractionHeavy()
  }
}
`

func LedgerHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(ledgerHeavyScriptTemplate, favContractAddress))
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
}

// TODO add tx size heavy similar to add keys
