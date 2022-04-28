import NonFungibleToken from 0x1d7e57aa55817448

pub contract BlockpartyNFT: NonFungibleToken {
    pub var totalSupply: UInt64

    // addresses that should be used to store account's collection 
    // and for interactions with it within transactions
    // WARNING: only resources of type BlockpartyNFT.Collection 
    //          should be stored by this paths.
    //          Storing resources of other types can lead to undefined behavior
    pub var BNFTCollectionStoragePath: StoragePath 
    pub var BNFTCollectionPublicPath: PublicPath
    pub var FullBNFTCollectionPublicPath: PublicPath

    // addresses that should be used use to store tokenD account's address. 
    // Only one tokenD address can be stored at a time. 
    // Address stored by this path is allowed to be overriden but 
    // be careful that after you override it new address will 
    // be used to all TokenD interactions 
    pub var TokenDAccountAddressProviderStoragePath: StoragePath
    pub var TokenDAccountAddressProviderPublicPath: PublicPath

    pub var AccountPreparedProviderStoragePath: StoragePath
    pub var AccountPreparedProviderPublicPath: PublicPath

    pub var IsStorageUpdatedToV1ProviderStoragePath: StoragePath
    pub var IsStorageUpdatedToV1ProviderPublicPath: PublicPath

    pub var MinterStoragePath: StoragePath

    // pub var adminPublicCollection: &AnyResource{NonFungibleToken.CollectionPublic}

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // Including `id` and addresses here to avoid complex event parsing logic
    pub event TransferredToServiceAccount(id: UInt64, from: Address, extSystemAddrToMint: String)
    pub event MintedFromWithdraw(id: UInt64, withdrawRequestID: UInt64, to: Address?)
    pub event MintedFromIssuance(id: UInt64, issuanceRequestID: UInt64, to: Address?)

    pub event Burned(id: UInt64)

    pub struct IssuanceMintMsg {
        pub let issuanceRequestID: UInt64
        pub let detailsURL: String

        init(id: UInt64, detailsURL: String) {
            self.issuanceRequestID = id
            self.detailsURL = detailsURL
        }
    }

    pub struct TokenDAddressProvider {
        pub let tokenDAddress: String
        pub init(tokenDAddress: String) {
            self.tokenDAddress = tokenDAddress
        }
    }

    pub struct AccountPreparedProvider { // TODO move to separate proxy contract
        pub var isPrepared: Bool
        pub init(isPrepared: Bool) {
            self.isPrepared = isPrepared
        }
        pub fun setPrepared(isPrepared: Bool) {
            self.isPrepared = isPrepared
        }
    }

    pub struct IsStorageUpdatedToV1Provider { // TODO move to separate proxy contract
        pub var isUpdated: Bool
        pub init(isUpdated: Bool) {
            self.isUpdated = isUpdated
        }
        pub fun setUpdated(isUpdated: Bool) {
            self.isUpdated = isUpdated
        }
    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let detailsURL: String

        init(id: UInt64, detailsURL: String) {
            self.id = id
            self.detailsURL = detailsURL
        }
    }

    pub resource interface BNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        pub fun borrowBNFT(id: UInt64): &BlockpartyNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow NFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, BNFTCollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        pub var tokenDDepositerCap: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>
        
        init(tokenDDepositerCap: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>) {
            self.tokenDDepositerCap = tokenDDepositerCap
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("no token found with provided withdrawID")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @BlockpartyNFT.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        pub fun depositToTokenD(id: UInt64) {
            if !self.tokenDDepositerCap.check() {
                panic("TokenD depositer cap not found. You either trying to deposit from admin account or something wrong with collection initialization")
            }

            let token <- self.withdraw(withdrawID: id)

            self.tokenDDepositerCap.borrow()!.deposit(token: <-token)

            let addrToIssueProvider = self.owner!.getCapability<&TokenDAddressProvider>(BlockpartyNFT.TokenDAccountAddressProviderPublicPath).borrow()!

            emit TransferredToServiceAccount(id: id, from: self.owner!.address, extSystemAddrToMint: addrToIssueProvider.tokenDAddress)
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowBNFT(id: UInt64): &BlockpartyNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &BlockpartyNFT.NFT
            }
            return nil
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub resource NFTMinter {
        access(self) var BNFTCollectionPublicPath: PublicPath

        init(collectionPublicPath: PublicPath) {
            self.BNFTCollectionPublicPath = collectionPublicPath
        }

        pub fun mintNFTByIssuance( 
            requests: [IssuanceMintMsg]
        ) {
            let minterOwner = self.owner ?? panic("could not get minter owner")

            let minterOwnerCollection = minterOwner.getCapability(self.BNFTCollectionPublicPath).borrow<&{NonFungibleToken.CollectionPublic}>()
                ?? panic("Could not get reference to the service account's NFT Collection")

            var creationID = BlockpartyNFT.totalSupply + 1 as UInt64

            BlockpartyNFT.totalSupply = BlockpartyNFT.totalSupply + UInt64(requests.length)

            for req in requests {
                let token <-create NFT(id: creationID, detailsURL: req.detailsURL)
                let id = token.id

                // deposit it in the recipient's account using their reference
                minterOwnerCollection.deposit(token: <-token)

                emit MintedFromIssuance(id: id, issuanceRequestID: req.issuanceRequestID, to: self.owner?.address)
                creationID = creationID + 1 as UInt64
            }
        }

        // TODO redesign it it operate with tokens stored in a vault of the account which is owner of the Minter resource
        pub fun mintNFT(
            withdrawRequestID: UInt64,
            detailsURL: String,
            receiver: Address
        ) {
            // Borrow the recipient's public NFT collection reference
            let recipientAccount = getAccount(receiver)

            let recipientCollection = recipientAccount
                .getCapability(self.BNFTCollectionPublicPath)
                .borrow<&{NonFungibleToken.CollectionPublic}>()
                ?? panic("Could not get receiver reference to the NFT Collection")

            // create token with provided name and data
            let token <-create NFT(id: BlockpartyNFT.totalSupply + 1 as UInt64, detailsURL: detailsURL)
            let id = token.id

            // deposit it in the recipient's account using their reference
            recipientCollection.deposit(token: <-token)

            BlockpartyNFT.totalSupply = BlockpartyNFT.totalSupply + 1 as UInt64
            emit MintedFromWithdraw(id: id, withdrawRequestID: withdrawRequestID, to: receiver)
        }
    }

    pub resource NFTBurner {
        pub fun burnNFT(token: @NonFungibleToken.NFT) {
            let id = token.id
            destroy token
            emit Burned(id: id)
        }
    }

    pub fun createAccountPreparedProvider(isPrepared: Bool): AccountPreparedProvider {
        return AccountPreparedProvider(isPrepared: isPrepared)
    }

    pub fun createIsStorageUpdatedToV1Provider(isUpdated: Bool): IsStorageUpdatedToV1Provider {
        return IsStorageUpdatedToV1Provider(isUpdated: isUpdated)
    }

    pub fun createTokenDAddressProvider(tokenDAddress: String): TokenDAddressProvider {
        return TokenDAddressProvider(tokenDAddress: tokenDAddress)
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection(tokenDDepositerCap: self.account.getCapability<&AnyResource{NonFungibleToken.CollectionPublic}>(self.BNFTCollectionPublicPath))
    }

    // public function that anyone can call to create a burner to burn their oun tokens
    pub fun createBurner(): @NFTBurner {
        return <- create NFTBurner()
    }

    init() {
        self.totalSupply = 265

        self.BNFTCollectionStoragePath = /storage/NFTCollection
        self.BNFTCollectionPublicPath = /public/NFTCollection
        self.FullBNFTCollectionPublicPath = /public/BNFTCollection

        self.MinterStoragePath = /storage/NFTMinter

        self.TokenDAccountAddressProviderStoragePath = /storage/tokenDAccountAddr
        self.TokenDAccountAddressProviderPublicPath = /public/tokenDAccountAddr

        self.AccountPreparedProviderStoragePath = /storage/accountPrepared
        self.AccountPreparedProviderPublicPath = /public/accountPrepared

        self.IsStorageUpdatedToV1ProviderStoragePath = /storage/isStorageUpdatedToV1
        self.IsStorageUpdatedToV1ProviderPublicPath = /public/isStorageUpdatedToV1

        // not linking it to public path to avoid unauthorized access attempts
        // TODO make minter internal and use in only within contract
        let existingMinter = self.account.borrow<&NFTMinter>(from: self.MinterStoragePath)
        if existingMinter == nil { 
            // in case when contract is being deployed after removal minter does already exist and no need to save it once more
            self.account.save(<-create NFTMinter(collectionPublicPath: self.BNFTCollectionPublicPath), to: self.MinterStoragePath)
        }

        let adminCollectionExists = self.account.getCapability<&AnyResource{NonFungibleToken.CollectionPublic}>(self.BNFTCollectionPublicPath).check()
        if !adminCollectionExists {
            self.account.save(<-self.createEmptyCollection(), to: self.BNFTCollectionStoragePath)
            // adminCollection <-! self.createEmptyCollection() as @BlockpartyNFT.Collection?
        }
        self.account.link<&AnyResource{NonFungibleToken.CollectionPublic}>(self.BNFTCollectionPublicPath, target: self.BNFTCollectionStoragePath)

        let accountPrepared = self.account.copy<&AccountPreparedProvider>(from: self.AccountPreparedProviderStoragePath)
        if accountPrepared != nil && !(accountPrepared!.isPrepared) {
            self.account.save(AccountPreparedProvider(isPrepared: true), to: self.AccountPreparedProviderStoragePath)
            self.account.link<&AccountPreparedProvider>(self.AccountPreparedProviderPublicPath, target: self.AccountPreparedProviderStoragePath)
        }
        
        let isStorageUpdatedToV1 = self.account.copy<&IsStorageUpdatedToV1Provider>(from: self.IsStorageUpdatedToV1ProviderStoragePath)
        if isStorageUpdatedToV1 != nil && !(isStorageUpdatedToV1!.isUpdated) {
            self.account.save(IsStorageUpdatedToV1Provider(isUpdated: true), to: self.IsStorageUpdatedToV1ProviderStoragePath)
            self.account.link<&IsStorageUpdatedToV1Provider>(self.IsStorageUpdatedToV1ProviderPublicPath, target: self.IsStorageUpdatedToV1ProviderStoragePath)
        }
        
        emit ContractInitialized()
    }
}
