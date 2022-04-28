import NonFungibleToken from 0x1d7e57aa55817448

pub contract TatumMultiNFT:NonFungibleToken {

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, type: String, to: Address)
    pub event MinterAdded(address: Address, type: String)

    pub var totalSupply: UInt64

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath
    pub let AdminMinterStoragePath: StoragePath

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        pub let type: String;
        pub let metadata: String

        init(initID: UInt64, url: String, type: String) {
            self.id = initID
            self.metadata = url
            self.type = type;
        }
    }

    pub resource interface TatumMultiNftCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun getIDsByType(type: String): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT
        pub fun borrowTatumNFT(id: UInt64, type: String): &TatumMultiNFT.NFT
    }

    pub resource Collection: TatumMultiNftCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var types: {String: Int}
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{ UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
            self.types = {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @TatumMultiNFT.NFT
            let id: UInt64 = token.id
            let type: String = token.type

            // find if there is already existing token type
            var x = self.types[type] ?? self.ownedNFTs.length
            if self.types[type] == nil {
                // there is no token of this type, we need to store the index for the later easy access
                self.types[type] = self.ownedNFTs.length
            }
            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDsByType(type: String): [UInt64] {
            if self.types[type] != nil {
                let x = self.types[type] ?? panic("No such type")
                let res:[UInt64] = []
                for e in self.ownedNFTs.keys {
                    let t = self.borrowTatumNFT(id: e, type: type)
                    if t.type == type {
                        res.append(e);
                    }
                }
                return res
            } else {
                return []
            }
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowTatumNFT(id: UInt64, type: String): &TatumMultiNFT.NFT {
            let x = self.types[type] ?? panic("No such token type.")
            let token = (&self.ownedNFTs[id] as auth &NonFungibleToken.NFT) as! &TatumMultiNFT.NFT
            if token.type != type {
                panic("Token doesnt have correct type.")
            }
            return token
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource AdminMinter {

        access(self) var minters: {Address: Int}

        init() {
            self.minters = {};
        }

        pub fun addMinter(minterAccount: AuthAccount, type: String) {
            if self.minters[minterAccount.address] == 1 {
                panic("Unable to add minter, already present as a minter for another token type.")
            }
            let minter <- create NFTMinter(type: type)
            emit MinterAdded(address: minterAccount.address, type: type)
            minterAccount.save(<-minter, to: TatumMultiNFT.MinterStoragePath)
            self.minters[minterAccount.address] = 1;
        }
    }

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter {

        // This minter is allowed to mint only tokens of this type
        pub let type: String;

        init(type: String) {
            self.type = type;
        }

        // mintNFT mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(recipient: &{TatumMultiNftCollectionPublic}, type: String, url: String, address: Address) {
            if self.type != type {
                panic("Unable to mint token for type, where this account is not a minter")
            }

            // create a new NFT
            var newNFT <- create NFT(initID: TatumMultiNFT.totalSupply, url: url, type: type)

            emit Minted(id: TatumMultiNFT.totalSupply, type: type, to: address)
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)

            TatumMultiNFT.totalSupply = TatumMultiNFT.totalSupply + 1 as UInt64
        }
    }

    init() {
        self.totalSupply = 0
        self.CollectionStoragePath = /storage/TatumNFTCollection
        self.CollectionPublicPath = /public/TatumNFTCollection
        self.MinterStoragePath = /storage/TatumNFTMinter
        self.AdminMinterStoragePath = /storage/TatumNFTAdminMinter

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&{TatumMultiNftCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        // Create a default admin Minter resource and save it to storage
        // Admin minter cannot mint new tokens, it can only add new minters with for new token types
        let minter <- create AdminMinter()
        self.account.save(<-minter, to: self.AdminMinterStoragePath)

        emit ContractInitialized()
    }
}