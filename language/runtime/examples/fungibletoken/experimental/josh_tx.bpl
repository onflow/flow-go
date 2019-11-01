// This is what a transaction for interacting with the Fungible Token in fung.bpl would look like


// The first example is what it would look like for the account that originally deploys the token contract.

// imports the BasicToken contract and the resources and functions it contains
import BasicToken from "fung.bpl"
import BasicToken.TokenPool from "fung.bpl"

// imports the contract interface and its associated resource interfaces from the other file
import FungibleToken from "fung.bpl"

transaction {

    let mintedTokens: TokenPool

    prepare(signer: Account, initialBalance: Int) {
        // Deploy the BasicToken contract to your account
        // this makes the functions within BasicToken available to be called
        signer.deploy(BasicToken(initialBalance: initialBalance))

        // stores an instance of the TokenPool resource to your account storage
        signer.types[TokenPool] <- create TokenPool()

        // mint the initial balance of tokens to your account and deposit it
        mintedTokens <- signer.types[BasicToken].mint(amount: initialBalance)
        signer.types[TokenPool].deposit(tokens: <- mintedTokens)

        // Stores a reference to the stored resource using only the provider interface
        // Provider interface will only be able to be accessed by you
        signer.types[FungibleToken.Provider] = 
            &signer.types[TokenPool] as FungibleToken.Provider

        // Stores another reference to the stored resource using only the receiver interface
        // Receiver interface will be published and therefore accessible by everyone
        // this only includes your balance and your deposit function
        signer.types[FungibleToken.Receiver] = 
            &signer.types[TokenPool] as FungibleToken.Receiver

        // publish the TokenPool type so that others can use it in their accounts to store tokens
        // TODO:  How do we publish a resource type without making it so anyone can call its functions?
        // needs answering
        publish signer.types[TokenPool]

        // Make the deployed interface to your resource publically available
        publish signer.types[FungibleToken.Receiver]	
    }	    
}

// The next transaction is what it would look like for the original token contract creator to
// mint new tokens
// imports the BasicToken contract and the resources and functions it contains
import BasicToken from "fung.bpl"
import BasicToken.TokenPool from "fung.bpl"

// imports the contract interface and its associated resource interfaces from the other file
import FungibleToken from "fung.bpl"

transaction {

    let mintedTokens: TokenPool

    prepare(signer: Account, mintingAmount: Int) {
        // mint new tokens and deposit them in your account
        mintedTokens <- signer.types[BasicToken].mint(amount: mintingAmount)
        signer.types[TokenPool].deposit(tokens: <- mintedTokens)
    }	    
}




//  The next transaction is what it would look like for someone external who wants to use the token that was
//  published in the transaction above.  These steps are for adding it to their account storage

// imports the TokenPool resource from the account that published them
import TokenPool from 0x02928374837383838383

// imports the contract interface and its associated resource interfaces from the account
import FungibleToken from 0x02928374837383838383

transaction {
    prepare(signer: Account) {

        // stores an instance of the TokenPool resource to your account storage
        signer.types[TokenPool] <- create TokenPool()

        // Stores a reference to the stored resource using only the provider interface
        // Provider interface will only be able to be accessed by you
        signer.types[FungibleToken.Provider] = 
            &signer.types[TokenPool] as FungibleToken.Provider

        // Stores another reference to the stored resource using only the receiver interface
        // Receiver interface will be published and therefore accessible by everyone
        // this only includes your balance and your deposit function
        signer.types[FungibleToken.Receiver] = 
            &signer.types[TokenPool] as FungibleToken.Receiver

        // Make the deployed interface to your resource publically available
        publish signer.types[FungibleToken.Receiver]	
    }	    
}



// The next transactions is what it would look like for someone who has set up their account
// with the token information to send tokens to another account who has published a 
// Receiver interface for their token

// You are importing the recipients Receiver interface so you can deposit tokens in their account
import FungibleToken.Receiver from 0xBEEF837483738383DEAD

// Also importing the provider interface from your account so that you can withdraw from your own
// Is this needed?
// import FungibleToken from {signer's account address}

transaction {

    // the resource object that will be holding funds as they are sent
    let sentFunds: TokenPool

    // prepare handles all the parts of the transaction that have to do with the sender's account
    prepare(signer: Account, amount: Int) {

        // Call the transaction signer's withdraw function, which moves a resource object
        // with the required funds to the sentFunds object

        // As the access is performed in the preparer,	       
        // the unpublished reference `ExampleToken.Provider`	     
        // can be accessed (if it exists)
        //	
        self.sentFunds <- signer.types[FungibleToken.Provider].withdraw(amount: amount)	
    }

    // execute handles the external function calls in the transaction
    // In this example, it is finally depositing the tokens into the receiver's account
    execute(to: Account) {
        // Deposit the amount withdrawn from the signer
        // in the recipient's token pool through the stored, published receiver in the recipient's account
        // As you can see, the sent Funds object is moved so it will be invalid after this use
        recipient.types[Receiver].deposit(tokens: <- self.sentFunds)
        
    }

}
