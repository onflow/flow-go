import NonFungibleToken from 0x1d7e57aa55817448

pub contract IrVoucher: NonFungibleToken {
    
    //------------------------------------------------------------
    // Events
    //------------------------------------------------------------

    // Contract Events
    //
    pub event ContractInitialized()

    // NFT Collection Events (inherited from NonFungibleToken)
    //
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // NFT Events
    //
    pub event NFTMinted(
        id: UInt64, 
        dropID: UInt32,
        serial: UInt32
    )
    pub event NFTBurned(
        id: UInt64
    )

    //------------------------------------------------------------
    // Named Values
    //------------------------------------------------------------

    // Paths
    //
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    //------------------------------------------------------------
    // Public Contract State
    //------------------------------------------------------------

    // Entity Counts
    //
    pub var totalSupply: UInt64 // (inherited from NonFungibleToken)


    //------------------------------------------------------------
    // IN|RIFT NFT
    //------------------------------------------------------------

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        
        pub let dropID: UInt32

        pub let serial: UInt32

        init(
            initID: UInt64, 
            dropID: UInt32,
            serial: UInt32
        ) {
            self.id = initID
            self.dropID = dropID
            self.serial = serial
        }
    }

    //------------------------------------------------------------
    // (Drop) Voucher NFT Collection
    //------------------------------------------------------------

    // A public collection interface that allows IN|RIFT Voucher NFTs to be borrowed
    //
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)

        pub fun getIDs(): [UInt64]

        pub fun idExists(id: UInt64): Bool

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        pub fun borrowVoucher(id: UInt64): &NFT?
    }

    // The definition of the Collection resource that
    // holds the Drops (NFTs) that a user owns
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // Initialize the NFTs field to an empty collection
        init () {
            self.ownedNFTs <- {}
        }

        // withdraw
        //
        // Function that removes an NFT from the collection
        // and moves it to the calling context
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // If the NFT isn't found, the transaction panics and reverts
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Missing NFT to withdraw")

            return <-token
        }

        // deposit
        //
        // Function that takes a NFT as an argument and
        // adds it to the collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @IrVoucher.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            destroy oldToken
        }

        // idExists checks to see if a NFT
        // with the given ID exists in the collection
        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }

        // getIDs returns an array of the IDs that are in the collection
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

        // borrowVoucher
        pub fun borrowVoucher(id: UInt64): &IrVoucher.NFT? {
             if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                
                return ref as! &IrVoucher.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // Allow everyone to create a empty IN|RIFT Voucher Collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // mintVoucher
    // allows us to create Vouchers from another contract
    // in this account. This is helpful for 
    // allowing AdminContract to mint Vouchers.
    //
    access(account) fun mintVoucher(
        dropID: UInt32,
        serial: UInt32
    ): @NFT {
        let minter <- create NFTMinter()

        let voucher <- minter.mintNFT(
            dropID: dropID,
            serial: serial
        )

        destroy minter

        return <- voucher
    }


    //------------------------------------------------------------
    // NFT Minter
    //------------------------------------------------------------

    pub resource NFTMinter {

        // mintNFT
        //
        // Function that mints a new NFT with a new ID
        // and returns it to the caller
        pub fun mintNFT(
            dropID: UInt32,
            serial: UInt32
        ): @IrVoucher.NFT {

            // Create a new NFT
            var newNFT <- create IrVoucher.NFT(
                initID: IrVoucher.totalSupply,
                dropID: dropID,
                serial: serial
            )

            // Increase Total Supply
            IrVoucher.totalSupply = IrVoucher.totalSupply + 1;

            emit NFTMinted(
                id: newNFT.id, 
                dropID: newNFT.dropID,
                serial: newNFT.serial
            )

            return <-newNFT
        }

    }

    init() {
        // Set the named paths 
        self.CollectionStoragePath = /storage/irDropVoucherCollectionV1
        self.CollectionPublicPath = /public/irDropVoucherCollectionV1
        self.MinterStoragePath = /storage/irDropVoucherMinterV1

        // Initialize the total supply
        self.totalSupply = 0

        // Store an empty Voucher Collection in account storage
        // & publish a public reference to the Voucher Collection in storage
        self.account.save(
            <-self.createEmptyCollection(), 
            to: self.CollectionStoragePath
        )

        self.account.link<&IrVoucher.Collection{NonFungibleToken.CollectionPublic, IrVoucher.CollectionPublic}>(
            self.CollectionPublicPath, 
            target: self.CollectionStoragePath
        )

        // Store minter resources in account storage
        self.account.save(
            <-create NFTMinter(), 
            to: self.MinterStoragePath
        )

        emit ContractInitialized()
	}
}