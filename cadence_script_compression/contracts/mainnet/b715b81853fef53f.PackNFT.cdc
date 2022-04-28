import Crypto
import NonFungibleToken from 0x1d7e57aa55817448
import IPackNFT from 0xb357442e10e629e2

pub contract PackNFT: NonFungibleToken, IPackNFT {

    pub var totalSupply: UInt64
    pub let version: String
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionIPackNFTPublicPath: PublicPath
    pub let OperatorStoragePath: StoragePath
    pub let OperatorPrivPath: PrivatePath

    // representation of the NFT in this contract to keep track of states
    access(contract) let packs: @{UInt64: Pack}

    pub event RevealRequest(id: UInt64, openRequest: Bool)
    pub event OpenRequest(id: UInt64)
    pub event Revealed(id: UInt64, salt: String, nfts: String)
    pub event Opened(id: UInt64)
    pub event Mint(id: UInt64, commitHash: String, distId: UInt64)
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub enum Status: UInt8 {
        pub case Sealed
        pub case Revealed
        pub case Opened
    }

    pub resource PackNFTOperator: IPackNFT.IOperator {

         pub fun mint(distId: UInt64, commitHash: String, issuer: Address): @NFT{
            let id = PackNFT.totalSupply + 1
            let nft <- create NFT(initID: id, commitHash: commitHash, issuer: issuer)
            PackNFT.totalSupply = PackNFT.totalSupply + 1
            let p  <-create Pack(commitHash: commitHash, issuer: issuer)
            PackNFT.packs[id] <-! p
            emit Mint(id: id, commitHash: commitHash, distId: distId)
            return <- nft
         }

        pub fun reveal(id: UInt64, nfts: [{IPackNFT.Collectible}], salt: String) {
            let p <- PackNFT.packs.remove(key: id) ?? panic("no such pack")
            p.reveal(id: id, nfts: nfts, salt: salt)
            PackNFT.packs[id] <-! p
        }

        pub fun open(id: UInt64, nfts: [{IPackNFT.Collectible}]) {
            let p <- PackNFT.packs.remove(key: id) ?? panic("no such pack")
            p.open(id: id, nfts: nfts)
            PackNFT.packs[id] <-! p
        }

         init(){}
    }

    pub resource Pack {
        pub let commitHash: String
        pub let issuer: Address
        pub var status: PackNFT.Status 
        pub var salt: String?

        pub fun verify(nftString: String): Bool {
            assert(self.status != PackNFT.Status.Sealed, message: "Pack not revealed yet")
            var hashString = self.salt!
            hashString = hashString.concat(",").concat(nftString)
            let hash = HashAlgorithm.SHA2_256.hash(hashString.utf8)
            assert(self.commitHash == String.encodeHex(hash), message: "CommitHash was not verified")
            return true
        }

        access(self) fun _verify(nfts: [{IPackNFT.Collectible}], salt: String, commitHash: String): String {
            var hashString = salt
            var nftString = nfts[0].hashString()
            var i = 1
            while i < nfts.length {
                let s = nfts[i].hashString()
                nftString = nftString.concat(",").concat(s)
                i = i + 1
            }
            hashString = hashString.concat(",").concat(nftString)
            let hash = HashAlgorithm.SHA2_256.hash(hashString.utf8)
            assert(self.commitHash == String.encodeHex(hash), message: "CommitHash was not verified")
            return nftString
        }

        access(contract) fun reveal(id: UInt64, nfts: [{IPackNFT.Collectible}], salt: String) {
            assert(self.status == PackNFT.Status.Sealed, message: "Pack status is not Sealed")
            let v = self._verify(nfts: nfts, salt: salt, commitHash: self.commitHash)
            self.salt = salt
            self.status = PackNFT.Status.Revealed 
            emit Revealed(id: id, salt: salt, nfts: v)
        }

        access(contract) fun open(id: UInt64, nfts: [{IPackNFT.Collectible}]) {
            assert(self.status == PackNFT.Status.Revealed, message: "Pack status is not Revealed")
            self._verify(nfts: nfts, salt: self.salt!, commitHash: self.commitHash)
            self.status = PackNFT.Status.Opened
            emit Opened(id: id)
        }

        init(commitHash: String, issuer: Address) {
            self.commitHash = commitHash
            self.issuer = issuer
            self.status = PackNFT.Status.Sealed 
            self.salt = nil
        }
    }

    pub resource NFT: NonFungibleToken.INFT, IPackNFT.IPackNFTToken, IPackNFT.IPackNFTOwnerOperator {
        pub let id: UInt64
        pub let commitHash: String
        pub let issuer: Address

        pub fun reveal(openRequest: Bool){
            PackNFT.revealRequest(id: self.id, openRequest: openRequest)
        }

        pub fun open(){
            PackNFT.openRequest(id: self.id)
        }

        init(initID: UInt64, commitHash: String, issuer: Address ) {
            self.id = initID
            self.commitHash = commitHash
            self.issuer = issuer
        }

    }

    pub resource Collection:
        NonFungibleToken.Provider,
        NonFungibleToken.Receiver,
        NonFungibleToken.CollectionPublic,
        IPackNFT.IPackNFTCollectionPublic
    {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <- token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @PackNFT.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowPackNFT(id: UInt64): &IPackNFT.NFT? {
            let nft<- self.ownedNFTs.remove(key: id) ?? panic("missing NFT")
            let token <- nft as! @PackNFT.NFT
            let ref = &token as &IPackNFT.NFT
            self.ownedNFTs[id] <-! token as! @PackNFT.NFT
            return ref
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    access(contract) fun revealRequest(id: UInt64, openRequest: Bool ) {
        let p = PackNFT.borrowPackRepresentation(id: id) ?? panic ("No such pack")
        assert(p.status == PackNFT.Status.Sealed, message: "Pack status must be Sealed for reveal request")
        emit RevealRequest(id: id, openRequest: openRequest)
    }

    access(contract) fun openRequest(id: UInt64) {
        let p = PackNFT.borrowPackRepresentation(id: id) ?? panic ("No such pack")
        assert(p.status == PackNFT.Status.Revealed, message: "Pack status must be Revealed for open request")
        emit OpenRequest(id: id)
    }

    pub fun publicReveal(id: UInt64, nfts: [{IPackNFT.Collectible}], salt: String) {
        let p = PackNFT.borrowPackRepresentation(id: id) ?? panic ("No such pack")
        p.reveal(id: id, nfts: nfts, salt: salt)
    }

    pub fun borrowPackRepresentation(id: UInt64):  &Pack? {
        return &self.packs[id] as &Pack
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    init(
        CollectionStoragePath: StoragePath,
        CollectionPublicPath: PublicPath,
        CollectionIPackNFTPublicPath: PublicPath,
        OperatorStoragePath: StoragePath,
        OperatorPrivPath: PrivatePath,
        version: String
    ){
        self.totalSupply = 0
        self.packs <- {}
        self.CollectionStoragePath = CollectionStoragePath
        self.CollectionPublicPath = CollectionPublicPath
        self.CollectionIPackNFTPublicPath = CollectionIPackNFTPublicPath
        self.OperatorStoragePath = OperatorStoragePath
        self.OperatorPrivPath = OperatorPrivPath
        self.version = version

        // Create a collection to receive Pack NFTs
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)
        self.account.link<&Collection{NonFungibleToken.CollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)
        self.account.link<&Collection{IPackNFT.IPackNFTCollectionPublic}>(self.CollectionIPackNFTPublicPath, target: self.CollectionStoragePath)

        // Create a operator to share mint capability with proxy
        let operator <- create PackNFTOperator()
        self.account.save(<-operator, to: self.OperatorStoragePath)
        self.account.link<&PackNFTOperator{IPackNFT.IOperator}>(self.OperatorPrivPath, target: self.OperatorStoragePath)
    }

}
