/*

# Fungible Token Private Receiver Contract

This contract implements a special resource and receiver interface
whose deposit function is only callable by an admin through a public capability.

*/

import FungibleToken from 0xf233dcee88fe0abe

pub contract PrivateReceiverForwarder {

    // Event that is emitted when tokens are deposited to the target receiver
    pub event PrivateDeposit(amount: UFix64, to: Address?)

    pub let SenderStoragePath: StoragePath

    pub let PrivateReceiverStoragePath: StoragePath
    pub let PrivateReceiverPublicPath: PublicPath

    pub resource Forwarder {

        // This is where the deposited tokens will be sent.
        // The type indicates that it is a reference to a receiver
        //
        access(self) var recipient: Capability<&{FungibleToken.Receiver}>

        // deposit
        //
        // Function that takes a Vault object as an argument and forwards
        // it to the recipient's Vault using the stored reference
        //
        access(contract) fun deposit(from: @FungibleToken.Vault) {
            let receiverRef = self.recipient.borrow()!

            let balance = from.balance

            receiverRef.deposit(from: <-from)

            emit PrivateDeposit(amount: balance, to: self.owner?.address)
        }

        init(recipient: Capability<&{FungibleToken.Receiver}>) {
            pre {
                recipient.borrow() != nil: "Could not borrow Receiver reference from the Capability"
            }
            self.recipient = recipient
        }
    }

    // createNewForwarder creates a new Forwarder reference with the provided recipient
    //
    pub fun createNewForwarder(recipient: Capability<&{FungibleToken.Receiver}>): @Forwarder {
        return <-create Forwarder(recipient: recipient)
    }


    pub resource Sender {
        pub fun sendPrivateTokens(_ address: Address, tokens: @FungibleToken.Vault) {

            let account = getAccount(address)

            let privateReceiver = account.getCapability<&PrivateReceiverForwarder.Forwarder>(PrivateReceiverForwarder.PrivateReceiverPublicPath)
                .borrow() ?? panic("Could not borrow reference to private forwarder")

            privateReceiver.deposit(from: <-tokens)

        }
    }

    init() {
        self.SenderStoragePath = /storage/privateForwardingSender
        self.PrivateReceiverStoragePath = /storage/privateForwardingStorage
        self.PrivateReceiverPublicPath = /public/privateForwardingPublic

        self.account.save(<-create Sender(), to: self.SenderStoragePath)

    }
}