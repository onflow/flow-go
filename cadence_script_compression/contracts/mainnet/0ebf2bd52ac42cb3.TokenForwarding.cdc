/**

# Fungible Token Forwarding Contract

This contract shows how an account could set up a custom FungibleToken Receiver
to allow them to forward tokens to a different account whenever they receive tokens.

They can publish this Forwarder resource as a Receiver capability just like a Vault,
and the sender doesn't even need to know it is different.

When an account wants to create a Forwarder, they call the createNewForwarder
function and provide it with the Receiver reference that they want to forward
their tokens to.

*/

import FungibleToken from 0xf233dcee88fe0abe

pub contract TokenForwarding {

    // Event that is emitted when tokens are deposited to the target receiver
    pub event ForwardedDeposit(amount: UFix64, from: Address?)

    pub resource Forwarder: FungibleToken.Receiver {

        // This is where the deposited tokens will be sent.
        // The type indicates that it is a reference to a receiver
        //
        access(self) var recipient: &{FungibleToken.Receiver}

        // deposit
        //
        // Function that takes a Vault object as an argument and forwards
        // it to the recipient's Vault using the stored reference
        //
        pub fun deposit(from: @FungibleToken.Vault) {
            emit ForwardedDeposit(amount: from.balance, from: self.owner?.address)
            self.recipient.deposit(from: <-from)
        }

        // changeRecipient changes the recipient of the forwarder to the provided recipient
        //
        pub fun changeRecipient(_ newRecipient: &{FungibleToken.Receiver}) {
            self.recipient = newRecipient
        }

        init(recipient: &{FungibleToken.Receiver}) {
            self.recipient = recipient
        }
    }

    // createNewForwarder creates a new Forwarder reference with the provided recipient
    //
    pub fun createNewForwarder(recipient: &{FungibleToken.Receiver}): @Forwarder {
        return <-create Forwarder(recipient: recipient)
    }
}
