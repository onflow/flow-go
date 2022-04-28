// SPDX-License-Identifier: Unlicense
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import Peakon from 0xf6cb854714273458

/*
    This is a "Rights Holder Splits" contract, that will be triggered upon completion of
    a Buy/Sell Clip Card smart contract.

    The main purpose is splitting sale payment comission to multiple recipients,
    i.e. fund the seller of the card, as well as the accounts for RP and YouTube.
*/

pub contract RightsHolderSplits {

    // Events
    //
    pub event ContractInitialized()

    // PaymentSplitProcessed
    //
    // The event that is emitted when tokens are deposited to a Vault
    pub event PaymentSplitProcessed(nftID: UInt64, amount: UFix64, to: Address?)

    // PaymentSplit
    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    //
    pub struct PaymentSplit {
        // The receiver for the payment.
        // Note that we do not store an address to find the Vault that this represents,
        // as the link or resource that we fetch in this way may be manipulated,
        // so to find the address that a cut goes to you must get this struct and then
        // call receiver.borrow()!.owner.address on it.
        // This can be done efficiently in a script.
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        // The amount of the payment FungibleToken that will be paid to the receiver.
        pub let amount: UFix64

        // The receiver address.
        pub let address: Address

        // initializer
        //
        init(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64, address: Address) {
            self.receiver = receiver
            self.amount = amount
            self.address = address
        }
    }

    pub fun execute(payment: @FungibleToken.Vault, nftID: UInt64, residualReceiver: &{FungibleToken.Receiver}, slipts: [PaymentSplit]) {
        // Pay each beneficiary their amount of the payment.
        for split in slipts {
            if let receiver = split.receiver.borrow() {
                let paymentCut <- payment.withdraw(amount: split.amount)
                receiver.deposit(from: <-paymentCut)
                emit PaymentSplitProcessed(nftID: nftID, amount: split.amount, to: split.address)
            }
        }

        // At this point, if all recievers were active and availabile, then the payment Vault will have
        // zero tokens left, and this will functionally be a no-op that consumes the empty vault
        residualReceiver!.deposit(from: <-payment)
    }

    init () {
        emit ContractInitialized()
    }
}
