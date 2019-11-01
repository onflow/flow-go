// This is what a transaction for interacting with the Fungible Token in single_interface.bpl would look like


// The first example is what it would look like for the account that originally deploys the token contract.

// imports the BasicToken contract and the resources and functions it contains


import IFungibleToken from "fungible_interfaces.bpl"
import IMintableToken from "fungible_interfaces.bpl"
import IApprovalToken from "fungible_interfaces.bpl"
import ITokenContract from "fungible_interfaces.bpl"
import BasicToken from "single_interface.bpl"

transaction {

    prepare(signer: Account, initialBalance: Int, symbol: String) {

        // Someone can publish an interface to their account if they want
        // to allow other developers to import an immutable version of it during development
        signer.publish(ITokenContract)

        signer.publish(IFungibleToken)

        signer.publish(IMintableToken)

        signer.publish(IBurnableToken)
        
        // deploy the `BasicToken` contract code which contains the `Tokens` resource definition
        // the interfaces that it implements are "built in" to the contract code and resource code
        // but the interfaces themselves are not stored separately
        // calls the constructor which initializes the fields in the contract
        // As part of the constructor, it stores a `Tokens` resource in the owner's account
        // that has balance equal to initialBalance
        signer.storage[BasicToken] = new BasicToken(initialBalance: initialBalance, symbol: String)

    }	    
}

// The next transaction is what it would look like for the original token contract creator to
// mint new tokens
// imports the BasicToken contract and the resources and functions it contains
// import BasicToken from "fung.bpl"
// import BasicToken.TokenPool from "fung.bpl"

transaction {

    prepare(signer: Account, mintingAmount: Int) {
        // mint new tokens and deposit them in your account
        signer.storage[BasicToken].mint(signer, mintingAmount)
    }
}




//  The next transaction is what it would look like for someone external who wants to use the token that was
//  published in the transaction above.  These steps are for adding it to their account storage

// imports a reference to the TokenPool resource from the account that published them
import Tokens from 0x02928374837383838383

transaction {
    prepare(signer: Account) {

        // stores an instance of the TokenPool resource to your account storage
        signer.storage[Tokens] = Tokens.create()
    }	    
}



// The next transactions is what it would look like for someone who has set up their account
// with the token information to send tokens to another account who has published a 
// Receiver interface for their token


// Also importing the provider interface from your account so that you can withdraw from your own
// Is this needed?
import Token from 0x0404030303

transaction {

    // prepare handles all the parts of the transaction that have to do with the sender's account
    prepare(signer: Account, to: Account, amount: Int) {
    }

    execute(signer: Account, to: Account, amount: Int) {
        signer.storage[Tokens].transfer(Token, amount)
    }

}



