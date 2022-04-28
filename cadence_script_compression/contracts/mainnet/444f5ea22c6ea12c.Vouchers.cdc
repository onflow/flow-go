import NonFungibleToken from 0x1d7e57aa55817448

pub contract Vouchers: NonFungibleToken {
    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)

    // Redeemed
    // Fires when a user Redeems a Voucher, prepping
    // it for Consumption to receive reward
    //
    pub event Redeemed(id: UInt64)

    // Consumed
    // Fires when an Admin consumes a Voucher, deleting it forever
    // NOTE: Reward is not tracked. This is to simplify contract.
    //       It is to be administered in the consume() tx, 
    //       else thoust be punished by thine users.
    //
    pub event Consumed(id: UInt64)

    // Voucher Collection Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // Contract-Singleton Redeemed Voucher Collection
    pub let RedeemedCollectionPublicPath: PublicPath
    pub let RedeemedCollectionStoragePath: StoragePath

    // AdminProxy Receiver
    pub let AdminProxyStoragePath: StoragePath
    pub let AdminProxyPublicPath: PublicPath

    // Contract Owner Root Administrator Resource
    pub let AdministratorStoragePath: StoragePath
    pub let AdministratorPrivatePath: PrivatePath

    // totalSupply
    // The total number of Vouchers that have been minted
    //
    pub var totalSupply: UInt64

    // metadata
    // the mapping of Voucher TypeID's to their respective Metadata
    //
    access(contract) var metadata: {UInt64: Metadata}

    // redeemed
    // tracks currently redeemed vouchers for consumption
    // 
    access(contract) var redeemers: {UInt64: Address}

    // Voucher Type Metadata Definitions
    // 
    pub struct Metadata {
        pub let name: String
        pub let description: String
        // MIME type: image/png, image/jpeg, video/mp4, audio/mpeg
        pub let mediaType: String 
        // IPFS storage hash
        pub let mediaHash: String
        // URI to NFT media - incase IPFS not in use/avail
        pub let mediaURI: String

        init(name: String, description: String, mediaType: String, mediaHash: String, mediaURI: String) {
            self.name = name
            self.description = description
            self.mediaType = mediaType
            self.mediaHash = mediaHash
            self.mediaURI = mediaURI
        }
    }

    /// redeem(token)
    /// This public function represents the core feature of this contract: redemptions.
    /// The NFT's, aka Vouchers, can be 'redeemed' into the RedeemedCollection, which
    /// will ultimately consume them to the tune of an externally agreed-upon reward.
    ///
    pub fun redeem(collection: &Vouchers.Collection, voucherID: UInt64) {
        // withdraw their voucher
        let token <- collection.withdraw(withdrawID: voucherID)
        
        // establish the receiver for Redeeming Vouchers
        let receiver = Vouchers.account.getCapability<&{Vouchers.CollectionPublic}>(Vouchers.RedeemedCollectionPublicPath).borrow()!
        
        // deposit for consumption
        receiver.deposit(token: <- token)

        // store who redeemed this voucher for consumer to reward
        Vouchers.redeemers[voucherID] = collection.owner!.address
        emit Redeemed(id:voucherID)
    }

    // NFT
    // Voucher
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The token's ID
        pub let id: UInt64

        // The token's typeID
        pub let typeID: UInt64

        // Expose metadata of this Voucher type
        //
        pub fun getMetadata(): Metadata? {
            return Vouchers.metadata[self.typeID]
        }

        // init
        //
        init(initID: UInt64, typeID: UInt64) {
            self.id = initID
            self.typeID = typeID
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowVoucher(id: UInt64): &Vouchers.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Vouchers reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // A collection of Vouchers NFTs owned by an account
    //
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
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
            let token <- token as! @Vouchers.NFT

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

        // borrowVoucher
        // Gets a reference to an NFT in the collection as a Vouchers.NFT,
        // exposing all of its fields.
        // This is safe as there are no functions that can be called on the Vouchers.
        //
        pub fun borrowVoucher(id: UInt64): &Vouchers.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Vouchers.NFT
            } else {
                return nil
            }
        }

        // destructor
        //
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

    // AdminUsers will create a Proxy and be granted
    // access to the Administrator resource through their receiver, which
    // they can then borrowSudo() to utilize
    //
    pub fun createAdminProxy(): @AdminProxy { 
        return <- create AdminProxy()
    }

    // public receiver for the Administrator capability
    //
    pub resource interface AdminProxyPublic {
        pub fun addCapability(_ cap: Capability<&Vouchers.Administrator>)
    }

    /// AdminProxy
    /// This is a simple receiver for the Administrator resource, which
    /// can be borrowed if capability has been established.
    ///
    pub resource AdminProxy: AdminProxyPublic {
        // requisite receiver of Administrator capability
        access(self) var sudo: Capability<&Vouchers.Administrator>?
        
        // initializer
        //
        init () {
            self.sudo = nil
        }

        // must receive capability to take administrator actions
        //
        pub fun addCapability(_ cap: Capability<&Vouchers.Administrator>){ 
            pre {
                cap.check() : "Invalid Administrator capability"
                self.sudo == nil : "Administrator capability already set"
            }
            self.sudo = cap
        }

        // borrow a reference to the Administrator
        // 
        pub fun borrowSudo(): &Vouchers.Administrator {
            pre {
                self.sudo != nil : "Your AdminProxy has no Administrator capabilities."
            }
            let sudoReference = self.sudo!.borrow()
                ?? panic("Your AdminProxy has no Administrator capabilities.")

            return sudoReference
        }
    }

    /// Administrator
    /// Deployer-owned resource that Privately grants Capabilities to Proxies
    /// Can Mint Voucher NFT's, register their Metadata, and Consume them from the Redeemed Collection
    pub resource Administrator {
        // mintNFT
        // Mints a new NFT with a new ID
        // and deposits it in the recipients collection using their collection reference
        //
        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64) {
            emit Minted(id: Vouchers.totalSupply)

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <- create Vouchers.NFT(initID: Vouchers.totalSupply, typeID: typeID))
            Vouchers.totalSupply = Vouchers.totalSupply + (1 as UInt64)
        }

        // batchMintNFT
        // Mints a batch of new NFTs
        // and deposits them in the recipients collection using their collection reference
        //
        pub fun batchMintNFT(recipient: &{NonFungibleToken.CollectionPublic}, typeID: UInt64, count: Int) {
            var index = 0
        
            while index < count {
                self.mintNFT(
                    recipient: recipient,
                    typeID: typeID
                )

                index = index + 1
            }
        }

        // registerMetadata
        // Registers metadata for a typeID
        //
        pub fun registerMetadata(typeID: UInt64, metadata: Metadata) {
            Vouchers.metadata[typeID] = metadata
        }

        // consume
        // consumes a Voucher from the Redeemed Collection by destroying it
        // NOTE: it is expected the consumer also rewards the redeemer their due
        //          in the case of this repository, an NFT is included in the consume transaction
        pub fun consume(_ voucherID: UInt64): Address {
            // grab the voucher from the redeemed collection
            let redeemedCollection = Vouchers.account.borrow<&Vouchers.Collection>(from: Vouchers.RedeemedCollectionStoragePath)!
            let voucher <- redeemedCollection.withdraw(withdrawID: voucherID)
            
            // discard the empty collection and the voucher
            destroy voucher

            emit Consumed(id:voucherID)
            return Vouchers.redeemers[voucherID]!
        }
    }

    // fetch
    // Get a reference to a Vouchers from an account's Collection, if available.
    // If an account does not have a Vouchers.Collection, panic.
    // If it has a collection but does not contain the itemID, return nil.
    // If it has a collection and that collection contains the itemID, return a reference to that.
    //
    pub fun fetch(_ from: Address, itemID: UInt64): &Vouchers.NFT? {
        let collection = getAccount(from)
            .getCapability(Vouchers.CollectionPublicPath)
            .borrow<&Vouchers.Collection>()
            ?? panic("Couldn't get collection")
        // We trust Vouchers.Collection.borrowVoucher to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowVoucher(id: itemID)
    }

    // getMetadata
    // Get the metadata for a specific  of Vouchers
    //
    pub fun getMetadata(typeID: UInt64): Metadata? {
        return Vouchers.metadata[typeID]
    }

    // initializer
    //
    init() {
        self.CollectionStoragePath = /storage/jambbLaunchVouchersCollection
        self.CollectionPublicPath = /public/jambbLaunchVouchersCollection
		
        // only one redeemedCollection should ever exist, in the deployer storage
        self.RedeemedCollectionStoragePath = /storage/jambbLaunchVouchersRedeemedCollection
        self.RedeemedCollectionPublicPath = /public/jambbLaunchVouchersRedeemedCollection
        
        // only one Administrator should ever exist, in deployer storage
        self.AdministratorStoragePath = /storage/jambbLaunchVouchersAdministrator
        self.AdministratorPrivatePath = /private/jambbLaunchVouchersAdministrator

        self.AdminProxyPublicPath = /public/jambbLaunchVouchersAdminProxy
        self.AdminProxyStoragePath = /storage/jambbLaunchVouchersAdminProxy

        // Initialize the total supply
        self.totalSupply = 0

        // Initialize predefined metadata
        self.metadata = {}
        self.redeemers = {}

        // Create a NFTAdministrator resource and save it to storage
        let admin <- create Administrator()
        self.account.save(<- admin, to: self.AdministratorStoragePath)
        // Link it to provide shareable access route to capabilities
        self.account.link<&Vouchers.Administrator>(self.AdministratorPrivatePath, target: self.AdministratorStoragePath)

        // this contract will hold a Collection that Vouchers can be deposited to and Admins can Consume them to grant rewards
        // to the depositing account
        let redeemedCollection <- create Collection()
        // establish the collection users redeem into
        self.account.save(<- redeemedCollection, to: self.RedeemedCollectionStoragePath) 
        // set up a public link to the redeemed collection so they can deposit/view
        self.account.link<&{Vouchers.CollectionPublic}>(self.RedeemedCollectionPublicPath, target: self.RedeemedCollectionStoragePath)
        // set up a private link to the redeemed collection as a resource, so 

        // create a personal collection just in case contract ever holds Vouchers to distribute later etc
        let collection <- create Collection()
        self.account.save(<- collection, to: self.CollectionStoragePath)
        self.account.link<&{Vouchers.CollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        
        emit ContractInitialized()
    }
}
