package utils

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
)

const tokenTransferTransactionTemplate = `
import FungibleToken from 0x02
import FlowToken from 0x03

transaction {
    let sentVault: @FungibleToken.Vault
    prepare(signer: AuthAccount) {
        let storedVault = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Unable to borrow a reference to the sender's Vault")
        self.sentVault <- storedVault.withdraw(amount: 10.0)
    }
    execute {
        let recipient = getAccount(0x04)
        let receiver = recipient
            .getCapability(/public/flowTokenReceiver)
            .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
            ?? panic("Unable to borrow receiver reference for recipient")
        receiver.deposit(from: <-self.sentVault)
    }
}
`

// TokenTransferScript returns a transaction script for transfering `amount` flow tokens to `toAddr` address
func TokenTransferScript(ftAddr, flowToken, toAddr *flowsdk.Address, amount float64) ([]byte, error) {
	withFTAddr := strings.ReplaceAll(string(tokenTransferTransactionTemplate), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)
	withAmount := strings.Replace(string(withToAddr), fmt.Sprintf("%f", amount), "0.01", 1)
	return []byte(withAmount), nil
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

//LockedTokenAccountCreationScriptLocalnet script to create a locked token account on localnet
const LockedTokenAccountCreationScriptLocalnet = `import FungibleToken from 0xee82856bf20e2aa6
 import FlowToken from 0x0ae53cb6e3f42a79
 import LockedTokens from 0xf8d6e0586b0a20c7
 
 /// Transaction that the main token admin would sign
 /// to create a shared account and an unlocked
 /// acount for a user
 
 transaction(
    fullAdminPublicKey: [UInt8], // Weight: 1000
    fullUserPublicKey: [UInt8], // Weight: 1000
 )  {
 
	 prepare(admin: AuthAccount) {
 
		 // Create the new accounts and add their keys
		 let sharedAccount = AuthAccount(payer: admin)
		 let userAccount = AuthAccount(payer: admin)
 
		 sharedAccount.addPublicKey(fullAdminPublicKey)

		 userAccount.addPublicKey(fullUserPublicKey)
 
		 // Create a private link to the stored vault
		 let vaultCapability = sharedAccount
			 .link<&FlowToken.Vault>(/private/flowTokenVault, target: /storage/flowTokenVault)
			 ?? panic("Could not link Flow Token Vault capability")
 
		 // create a locked token manager and stored it in the shared account
		 let lockedTokenManager <- LockedTokens.createLockedTokenManager(vault: vaultCapability)
		 sharedAccount.save(<-lockedTokenManager, to: LockedTokens.LockedTokenManagerStoragePath)
 
		 let tokenManagerCapability = sharedAccount
			 .link<&LockedTokens.LockedTokenManager>(
				 LockedTokens.LockedTokenManagerPrivatePath,
				 target: LockedTokens.LockedTokenManagerStoragePath
		 )   ?? panic("Could not link token manager capability")
 
		 let tokenHolder <- LockedTokens.createTokenHolder(lockedAddress: sharedAccount.address, tokenManager: tokenManagerCapability)
 
		 userAccount.save(
			 <-tokenHolder, 
			 to: LockedTokens.TokenHolderStoragePath,
		 )
 
		 userAccount.link<&LockedTokens.TokenHolder{LockedTokens.LockedAccountInfo}>(LockedTokens.LockedAccountInfoPublicPath, target: LockedTokens.TokenHolderStoragePath)
 
		 let tokenAdminCapability = sharedAccount
			 .link<&LockedTokens.LockedTokenManager>(
				 LockedTokens.LockedTokenAdminPrivatePath,
				 target: LockedTokens.LockedTokenManagerStoragePath)
			 ?? panic("Could not link token admin to token manager")
 
		 let tokenAdminCollection = admin
			 .borrow<&LockedTokens.TokenAdminCollection>(from: LockedTokens.LockedTokenAdminCollectionStoragePath)
			 ?? panic("Could not borrow reference to admin collection")
 
		 tokenAdminCollection.addAccount(sharedAccountAddress: sharedAccount.address, unlockedAccountAddress: userAccount.address, tokenAdmin: tokenAdminCapability)
 
		 // Override the default FlowToken receiver
		 sharedAccount.unlink(/public/flowTokenReceiver)
			 
		 // create new receiver that marks received tokens as unlocked
		 sharedAccount.link<&AnyResource{FungibleToken.Receiver}>(
			 /public/flowTokenReceiver,
			 target: LockedTokens.LockedTokenManagerStoragePath
		 )
 
		 // put normal receiver in a separate unique path
		 sharedAccount.link<&AnyResource{FungibleToken.Receiver}>(
			 /public/lockedFlowTokenReceiver,
			 target: /storage/flowTokenVault
		 )
	 }
 }
 `

//LockedTransferScriptLocalnet script to transfer locked tokens on localnet
const LockedTransferScriptLocalnet = `import FungibleToken from 0xee82856bf20e2aa6
import FlowToken from 0x0ae53cb6e3f42a79
import LockedTokens from 0xf8d6e0586b0a20c7

// Transaction for the token admin to send locked tokens to a locked account
// The account must have already been registerd with the token admin
// and the account must have a FlowToken Receiver at /public/lockedFlowTokenReceiver

transaction (recipient: Address, amount: UFix64) {

    // reference for the token admin collection that registers locked accounts
    let tokenAdminRef: &LockedTokens.TokenAdminCollection

    // The Vault resource that holds the tokens that are being transferred
    let sentVault: @FungibleToken.Vault

    // The address of the locked account
    let lockedAddress: Address

    // The balance of the locked account
    let lockedBalance: UFix64

    // FlowToken receiver for the locked account
    let lockedReceiver: &AnyResource{FungibleToken.Receiver}

    prepare(signer: AuthAccount) {
        // Get a reference to the signer's stored vault
        let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
			?? panic("Could not borrow reference to the owner's Vault!")

        self.tokenAdminRef = signer.borrow<&LockedTokens.TokenAdminCollection>(from: LockedTokens.LockedTokenAdminCollectionStoragePath)
            ?? panic("Could not borrow a reference to the locked token admin collection")

        // Withdraw tokens from the signer's stored vault
        self.sentVault <- vaultRef.withdraw(amount: amount)

        let lockedAccountInfoRef = getAccount(recipient)
            .getCapability<&LockedTokens.TokenHolder{LockedTokens.LockedAccountInfo}>(LockedTokens.LockedAccountInfoPublicPath)!
            .borrow() ?? panic("Could not borrow a reference to public LockedAccountInfo")

        self.lockedBalance = lockedAccountInfoRef.getLockedAccountBalance()

        self.lockedAddress = lockedAccountInfoRef.getLockedAccountAddress()

        self.lockedReceiver = getAccount(self.lockedAddress)
          .getCapability(/public/lockedFlowTokenReceiver)!
          .borrow<&{FungibleToken.Receiver}>()
          ?? panic("Unable to borrow receiver reference")
    }

    pre {
        self.tokenAdminRef.getAccount(address: self.lockedAddress) != nil: "The specified account is not a locked account registered with the token admin"
        self.lockedBalance == 0.0 : "Locked account must be empty"
    }

    execute {
        // Deposit the withdrawn tokens in the recipient's locked tokens receiver
        self.lockedReceiver.deposit(from: <-self.sentVault)
    }
}
`
