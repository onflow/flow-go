
import NonFungibleToken from 0x1d7e57aa55817448

pub contract NFTCollections {

    pub let version: UInt32
    pub let NFT_COLLECTION_MANAGER_PATH: StoragePath

    pub event ContractInitialized()
    pub event Withdraw(address: Address, id: UInt64)
    pub event Deposit(address: Address, id: UInt64)

    init() {
        self.version = 1
        self.NFT_COLLECTION_MANAGER_PATH = /storage/NFTCollectionManager
        emit ContractInitialized()
    }

    pub fun getVersion(): UInt32 {
        return self.version
    }

    pub resource interface WrappedNFT {
        pub fun getContractName(): String
        pub fun getAddress(): Address
        pub fun getCollectionPath(): PublicPath
        pub fun borrowNFT(): &NonFungibleToken.NFT
    }

    pub resource interface Provider {
        pub fun withdraw(address: Address, withdrawID: UInt64): @NonFungibleToken.NFT
        pub fun withdrawWrapper(address: Address, withdrawID: UInt64): @NFTWrapper
        pub fun batchWithdraw(address: Address, batch: [UInt64], into: &NonFungibleToken.Collection)
        pub fun batchWithdrawWrappers(address: Address, batch: [UInt64]): @Collection
        pub fun borrowWrapper(address: Address, id: UInt64): &NFTWrapper
        pub fun borrowNFT(address: Address, id: UInt64): &NonFungibleToken.NFT
    }

    pub resource interface Receiver {
        pub fun deposit(contractName: String, address: Address, collectionPath: PublicPath, token: @NonFungibleToken.NFT)
        pub fun depositWrapper(wrapper: @NFTWrapper)
        pub fun batchDeposit(contractName: String, address: Address, collectionPath: PublicPath, batch: @NonFungibleToken.Collection)
        pub fun batchDepositWrapper(batch: @Collection)
    }

    pub resource interface CollectionPublic {
        pub fun borrowWrapper(address: Address, id: UInt64): &NFTWrapper
        pub fun borrowNFT(address: Address, id: UInt64): &NonFungibleToken.NFT

        pub fun deposit(contractName: String, address: Address, collectionPath: PublicPath, token: @NonFungibleToken.NFT)
        pub fun depositWrapper(wrapper: @NFTWrapper)
        pub fun batchDeposit(contractName: String, address: Address, collectionPath: PublicPath, batch: @NonFungibleToken.Collection)
        pub fun batchDepositWrapper(batch: @Collection)

        pub fun getIDs(): {Address:[UInt64]}
    }

    // A resource for managing collections of NFTs
    //
    pub resource NFTCollectionManager {

        access(self) let collections: @{String: Collection}

        init() {
            self.collections <- {}
        }

        pub fun getCollectionNames(): [String] {
            return self.collections.keys
        }

        pub fun createOrBorrowCollection(_ namespace: String): &Collection {
            if self.collections[namespace] == nil {
                return self.createCollection(namespace)
            } else {
                return self.borrowCollection(namespace)
            }
        }

        pub fun createCollection(_ namespace: String): &Collection {
            pre {
                self.collections[namespace] == nil: "Collection with that namespace already exists"
            }
            let alwaysEmpty <- self.collections[namespace] <- create Collection()
            destroy alwaysEmpty
            return &self.collections[namespace] as &Collection
        }

        pub fun borrowCollection(_ namespace: String): &Collection {
            pre {
                self.collections[namespace] != nil: "Collection with that namespace not found"
            }
            return &self.collections[namespace] as &Collection
        }

        destroy() {
            destroy self.collections
        }
    }

    // Creates and returns a new NFTCollectionManager resource for managing many
    // different Collections
    //
    pub fun createNewNFTCollectionManager(): @NFTCollectionManager {
        return <- create NFTCollectionManager()
    }

    // An NFT wrapped with useful information
    //
    pub resource NFTWrapper : WrappedNFT {
        access(self) let contractName: String
        access(self) let address: Address
        access(self) let collectionPath: PublicPath
        access(self) var nft: @NonFungibleToken.NFT?

        init(
            contractName: String,
            address: Address,
            collectionPath: PublicPath,
            token: @NonFungibleToken.NFT
        ) {
            self.contractName = contractName
            self.address = address
            self.collectionPath = collectionPath
            self.nft <- token
        }

        pub fun getContractName(): String {
            return self.contractName
        }

        pub fun getAddress(): Address {
            return self.address
        }

        pub fun getCollectionPath(): PublicPath {
            return self.collectionPath
        }

        pub fun borrowNFT(): &NonFungibleToken.NFT {
            pre {
                self.nft != nil: "Wrapped NFT is nil"
            }
            let optionalNft <- self.nft <- nil
            let nft <- optionalNft!!
            let ret = &nft as &NonFungibleToken.NFT
            self.nft <-! nft
            return ret!!
        }

        access(contract) fun unwrapNFT(): @NonFungibleToken.NFT {
            pre {
                self.nft != nil: "Wrapped NFT is nil"
            }
            let optionalNft <- self.nft <- nil
            let nft <- optionalNft!!
            return <- nft
        }

        destroy() {
            pre {
                self.nft == nil: "Wrapped NFT is not nil"
            }
            destroy self.nft
        }
    }

    pub resource Collection : CollectionPublic, Provider, Receiver {

        access(self) var collections: @{Address: ShardedNFTWrapperCollection}

        init() {
            self.collections <- {}
        }

        pub fun deposit(contractName: String, address: Address, collectionPath: PublicPath, token: @NonFungibleToken.NFT) {
            let wrapper <- create NFTWrapper(
                contractName: contractName,
                address: address,
                collectionPath: collectionPath,
                token: <- token
            )

            if self.collections[address] == nil {
                self.collections[address] <-! NFTCollections.createEmptyShardedNFTWrapperCollection()
            }

            let collection <- self.collections.remove(key: address)!
            collection.deposit(wrapper: <- wrapper)
            self.collections[address] <-! collection
        }

        pub fun depositWrapper(wrapper: @NFTWrapper) {
            let address = wrapper.getAddress()
            if self.collections[address] == nil {
                self.collections[address] <-! NFTCollections.createEmptyShardedNFTWrapperCollection()
            }

            let collection <- self.collections.remove(key: address)!
            collection.deposit(wrapper: <- wrapper)
            self.collections[address] <-! collection
        }

        pub fun batchDeposit(contractName: String, address: Address, collectionPath: PublicPath, batch: @NonFungibleToken.Collection) {
            let keys = batch.getIDs()
            for key in keys {
                self.deposit(
                    contractName: contractName,
                    address: address,
                    collectionPath: collectionPath,
                    token: <- batch.withdraw(withdrawID: key)
                )
            }
            destroy batch
        }

        pub fun batchDepositWrapper(batch: @Collection) {
            var addressMap = batch.getIDs()
            for address in addressMap.keys {
                let ids = addressMap[address] ?? []
                for id in ids {
                    self.depositWrapper(
                        wrapper: <- batch.withdrawWrapper(
                            address: address,
                            withdrawID: id
                        )
                    )
                }
            }
            destroy batch
        }

        pub fun withdraw(address: Address, withdrawID: UInt64): @NonFungibleToken.NFT {
            if self.collections[address] == nil {
                panic("No NFT with that Address exists")
            }

            let collection <- self.collections.remove(key: address)!
            let wrapper <- collection.withdraw(withdrawID: withdrawID)
            self.collections[address] <-! collection
            let nft <- wrapper.unwrapNFT()
            destroy wrapper
            return <- nft
        }

        pub fun withdrawWrapper(address: Address, withdrawID: UInt64): @NFTWrapper {
            if self.collections[address] == nil {
                panic("No NFT with that Address exists")
            }

            let collection <- self.collections.remove(key: address)!
            let wrapper <- collection.withdraw(withdrawID: withdrawID)
            self.collections[address] <-! collection
            return <- wrapper
        }

        pub fun batchWithdraw(address: Address, batch: [UInt64], into: &NonFungibleToken.Collection) {
            for id in batch {
                into.deposit(
                    token: <- self.withdraw(
                        address: address,
                        withdrawID: id
                    )
                )
            }
        }

        pub fun batchWithdrawWrappers(address: Address, batch: [UInt64]): @Collection {
            var into <- NFTCollections.createEmptyCollection()
            for id in batch {
                into.depositWrapper(
                    wrapper: <- self.withdrawWrapper(
                        address: address,
                        withdrawID: id
                    )
                )
            }
            return <- into
        }

        pub fun getIDs(): {Address:[UInt64]} {
            var ids: {Address:[UInt64]} = {}
            for key in self.collections.keys {
                ids[key] = []
                for id in self.collections[key]?.getIDs() ?? [] {
                    ids[key]!.append(id)
                }
            }
            return ids
        }

        pub fun borrowNFT(address: Address, id: UInt64): &NonFungibleToken.NFT {
            return self.borrowWrapper(address: address, id: id)!.borrowNFT()
        }

        pub fun borrowWrapper(address: Address, id: UInt64): &NFTWrapper {
            if self.collections[address] == nil {
                panic("No NFT with that Address exists")
            }
            let collection = &self.collections[address] as &ShardedNFTWrapperCollection
            return collection.borrowWrapper(id: id)
        }

        destroy() {
            destroy self.collections
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create NFTCollections.Collection()
    }

    pub resource ShardedNFTWrapperCollection {

        access(self) var collections: @{UInt64: NFTWrapperCollection}
        access(self) let numBuckets: UInt64

        init(numBuckets: UInt64) {
            self.collections <- {}
            self.numBuckets = numBuckets
            var i: UInt64 = 0
            while i < numBuckets {
                self.collections[i] <-! NFTCollections.createEmptyNFTWrapperCollection() as! @NFTWrapperCollection
                i = i + UInt64(1)
            }
        }

        pub fun deposit(wrapper: @NFTWrapper) {
            let bucket = wrapper.borrowNFT().id % self.numBuckets
            let collection <- self.collections.remove(key: bucket)!
            collection.deposit(wrapper: <-wrapper)
            self.collections[bucket] <-! collection
        }

        pub fun batchDeposit(batch: @ShardedNFTWrapperCollection) {
            let keys = batch.getIDs()
            for key in keys {
                self.deposit(wrapper: <- batch.withdraw(withdrawID: key))
            }
            destroy batch
        }

        pub fun withdraw(withdrawID: UInt64): @NFTWrapper {
            let bucket = withdrawID % self.numBuckets
            let wrapper <- self.collections[bucket]?.withdraw(withdrawID: withdrawID)!
            return <-wrapper
        }

        pub fun batchWithdraw(batch: [UInt64]): @ShardedNFTWrapperCollection {
            var batchCollection <- NFTCollections.createEmptyShardedNFTWrapperCollection()
            for id in batch {
                batchCollection.deposit(wrapper: <-self.withdraw(withdrawID: id))
            }
            return <- batchCollection
        }

        pub fun getIDs(): [UInt64] {
            var ids: [UInt64] = []
            for key in self.collections.keys {
                for id in self.collections[key]?.getIDs() ?? [] {
                    ids.append(id)
                }
            }
            return ids
        }

        pub fun borrowWrapper(id: UInt64): &NFTWrapper {
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.borrowWrapper(id: id)!
        }

        destroy() {
            destroy self.collections
        }
    }

    pub fun createEmptyShardedNFTWrapperCollection(): @ShardedNFTWrapperCollection {
        return <- create NFTCollections.ShardedNFTWrapperCollection(32)
    }

    // A collection of NFTWrappers
    //
    pub resource NFTWrapperCollection {

        access(self) var wrappers: @{UInt64: NFTWrapper}

        init() {
            self.wrappers <- {}
        }

        pub fun deposit(wrapper: @NFTWrapper) {
            let address = wrapper.getAddress()
            let id = wrapper.borrowNFT().id
            let oldWrapper <- self.wrappers[id] <- wrapper
            if oldWrapper != nil {
                panic("This Collection already has an NFTWrapper with that id")
            }
            emit Deposit(address: address, id: id)
            destroy oldWrapper
        }

        pub fun batchDeposit(batch: @NFTWrapperCollection) {
            let keys = batch.getIDs()
            for key in keys {
                self.deposit(wrapper: <-batch.withdraw(withdrawID: key))
            }
            destroy batch
        }

        pub fun withdraw(withdrawID: UInt64): @NFTWrapper {
            let wrapper <- self.wrappers.remove(key: withdrawID)
                ?? panic("Cannot withdraw: NFTWrapper does not exist in the collection")
            emit Withdraw(address: wrapper.getAddress(), id: withdrawID)
            return <- wrapper
        }

        pub fun batchWithdraw(batch: [UInt64]): @NFTWrapperCollection {
            var batchCollection <- create NFTWrapperCollection()
            for id in batch {
                batchCollection.deposit(wrapper: <-self.withdraw(withdrawID: id))
            }
            return <- batchCollection
        }

        pub fun getIDs(): [UInt64] {
            return self.wrappers.keys
        }

        pub fun borrowWrapper(id: UInt64): &NFTWrapper {
            return &self.wrappers[id] as &NFTWrapper
        }

        destroy() {
            destroy self.wrappers
        }
    }

    pub fun createEmptyNFTWrapperCollection(): @NFTWrapperCollection {
        return <- create NFTCollections.NFTWrapperCollection()
    }
}
