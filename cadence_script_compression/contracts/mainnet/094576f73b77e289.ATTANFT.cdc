import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61

pub contract ATTANFT: NonFungibleToken {
    // Events
    //
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, metadata: {String: String})
    pub event UserMinted(id: UInt64, price: UFix64)
    pub event PauseStateChanged(flag: Bool)
    pub event BaseURIChanged(uri: String)
    pub event MintPriceChanged(price: UFix64)

    // Named Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath


    // totalSupply
    // The total number of ATTANFT that have been minted
    //
    pub var totalSupply: UInt64

    pub var baseURI: String

    priv var pause: Bool

    priv var mintPrice: UFix64

    // NFT
    // A ATTA as an NFT
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64

        priv let metadata: {String: String}
        
        pub fun getMetadata(): {String: String} {
            return self.metadata
        }
        // initializer
        //
        init(initID: UInt64, metadata: {String: String}? ) {
            self.id = initID
            self.metadata = metadata ?? {}
        }
    }

    // The details of ATTA in the Collection.
    pub resource interface ATTACollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowATTA(id: UInt64): &ATTANFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow ATTA reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of ATTA NFTs owned by an account
    //
    pub resource Collection: ATTACollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        //
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // withdraw
        // Removes an NFT from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit
        // Takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @ATTANFT.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs
        // Returns an array of the IDs that are in the collection
        //
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowATTA
        // Gets a reference to an NFT in the collection as a ATTANFT,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the ATTANFT.
        //
        pub fun borrowATTA(id: UInt64): &ATTANFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ATTANFT.NFT
            } else {
                return nil
            }
        }

        // destructor
        destroy() {
            destroy self.ownedNFTs
        }

        // initializer
        //
        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // public function that anyone can call to create a new empty collection
    //
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // buy ATTA NFT with FLOW token
    // public function that anyone can call to buy a ATTA NFT with set price
    //
    pub fun buyATTA(paymentVault: @FungibleToken.Vault): @ATTANFT.NFT {
        pre {
            self.pause == false : "Mint pause."
            self.mintPrice > 0.0 : "Price not set yet."
        }
        // get user payment amount
        let paymentAmount = paymentVault.balance
        
        // check the amount enough or not
        if self.mintPrice > paymentAmount {
            panic("Not enough amount to mint a ATTA NFT")
        }
        // borrpw admin resource to receieve fund
        let admin = ATTANFT.account.borrow<&ATTANFT.Admin>(from: ATTANFT.AdminStoragePath) ?? panic("Could not borrow admin client")
        
        // keep the fund with admin resource's vault 
        admin.depositVault(paymentVault: <- paymentVault)
        
        emit UserMinted(id: ATTANFT.totalSupply, price: ATTANFT.mintPrice)

        // mint NFT and return
        let nft <- create ATTANFT.NFT(initID: ATTANFT.totalSupply, metadata: {})

        ATTANFT.totalSupply = ATTANFT.totalSupply + (1 as UInt64)

        return <- nft
    }

    // admin resource store in the contract owner's account with private path
    // this is a private resource that keep the mint and vault function for the admin
    //
    pub resource Admin {

        // vault receieve FLOW token 
        priv var vault: @FungibleToken.Vault

        // mint NFT without pay any FLOW for admin only
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: {String:String}? ) {
            emit Minted(id: ATTANFT.totalSupply, metadata: metadata ?? {})

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create ATTANFT.NFT(initID: ATTANFT.totalSupply, metadata: metadata))

            ATTANFT.totalSupply = ATTANFT.totalSupply + (1 as UInt64)
        }

        // deposite FLOW when user buy NFT with `buyATTA` function
        pub fun depositVault(paymentVault: @FungibleToken.Vault) {
            self.vault.deposit(from: <- paymentVault )
        }

        // global pause status, for emergency
        pub fun setPause(_ flag: Bool) {
            ATTANFT.pause = flag
            emit PauseStateChanged(flag: flag)
        }

        // baseURI field for the NFT metadata ,maintenaed off-chain
        pub fun setBaseURI(_ uri: String) {
            ATTANFT.baseURI = uri
            emit BaseURIChanged(uri: uri)
        }

        // set the NFT price to sell, user can buy nft by buyATTA function when price > 0 
        pub fun setPrice(_ price: UFix64) {
            ATTANFT.mintPrice = price
            emit MintPriceChanged(price: price)
        }

        // query FLOW vault balance of admin vault
        pub fun getVaultBalance():UFix64 {
            pre {
                self.vault != nil : "Vault not init yet..."
            }
           return self.vault.balance
        }

        // withdraw FLOW from admin's flow vault
        pub fun withdrawVault(amount: UFix64): @FungibleToken.Vault {
            pre {
                self.vault != nil : "Vault not init yet..."
            }
            let vaultRef = &self.vault as! &FungibleToken.Vault
            return <- vaultRef.withdraw(amount: amount)
        }
        
        init(vault: @FungibleToken.Vault) {
            
            self.vault <- vault
        }

        destroy() {
            destroy self.vault
        }

    }

    // query price 
    pub fun getPrice(): UFix64{
        return self.mintPrice
    }

    // query pause status
    pub fun isPause(): Bool{
        return self.pause
    }

    // query the BaseURI 
    pub fun getBaseURI(): String{
        return self.baseURI
    }

    // query admin FLOW balance with pub function
    pub fun getVaultBalance(): UFix64 {

        let admin = ATTANFT.account.borrow<&ATTANFT.Admin>(from: ATTANFT.AdminStoragePath) ?? panic("Could not borrow admin client")

        return admin.getVaultBalance()
    }

    // fetch
    // Get a reference to a ATTA from an account's Collection, if available.
    // If an account does not have a ATTANFT.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &ATTANFT.NFT? {
        let collection = getAccount(from)
            .getCapability(ATTANFT.CollectionPublicPath)!
            .borrow<&ATTANFT.Collection{ATTANFT.ATTACollectionPublic}>()
            ?? panic("Couldn't get collection")
        return collection.borrowATTA(id: itemID)
    }

    // initializer
    //
	init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/ATTANFTCollection
        self.CollectionPublicPath = /public/ATTANFTCollection
        self.AdminStoragePath = /storage/ATTANFTAdmin

        // Initialize the total supply
        self.totalSupply = 0
        self.baseURI = ""
        self.pause = true
        self.mintPrice = 0.0
        // Create a Minter resource and save it to storage
        let admin <- create Admin(vault: <- FlowToken.createEmptyVault())
        self.account.save(<-admin, to: self.AdminStoragePath)
        emit ContractInitialized()
	}
}
