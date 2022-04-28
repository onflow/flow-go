import NonFungibleToken from 0x1d7e57aa55817448

/**

A Metapier launchpad owner pass is an NFT that can be sent to
a project/token owner. The holder of this pass may execute 
functions designed for project owners in the corresponding 
launchpad pool.

For example, when the funding period is finished, the holder of
this pass may withdraw all the funds raised by the pool.

 */
pub contract MetapierLaunchpadOwnerPass: NonFungibleToken {

    pub event ContractInitialized()

    pub event Withdraw(id: UInt64, from: Address?)

    pub event Deposit(id: UInt64, to: Address?)

    pub event PassMinted(id: UInt64, launchPoolId: String, for: Address)

    pub var totalSupply: UInt64
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let launchPoolId: String

        init(id: UInt64, launchPoolId: String) {
            self.id = id
            self.launchPoolId = launchPoolId
        }
    }

    pub resource interface CollectionPublic {
        pub fun idExists(id: UInt64): Bool
        pub fun getIdsByPoolId(poolId: String): [UInt64]
    }

    pub resource Collection: 
        NonFungibleToken.Provider,
        NonFungibleToken.Receiver,
        NonFungibleToken.CollectionPublic ,
        CollectionPublic
    {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // If the NFT isn't found, the transaction panics and reverts
            let token <- self.ownedNFTs.remove(key: withdrawID)!

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            // make sure the token has the right type
            let token <- token as! @MetapierLaunchpadOwnerPass.NFT
            let id: UInt64 = token.id
            
            // add the new token to the dictionary with a force assignment
            // if there is already a value at that key, it will fail and revert
            self.ownedNFTs[id] <-! token

            emit Deposit(id: id, to: self.owner?.address)
        }

        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowPrivatePass(id: UInt64): &MetapierLaunchpadOwnerPass.NFT {
            let passRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return passRef as! &MetapierLaunchpadOwnerPass.NFT
        }

        pub fun getIdsByPoolId(poolId: String): [UInt64] {
            let ids: [UInt64] = []
            
            for key in self.ownedNFTs.keys {
                let passRef = self.borrowPrivatePass(id: key)

                if passRef.launchPoolId == poolId {
                    ids.append(key)
                }
            }

            return ids
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    pub resource Minter {
        pub fun mintNFT(recipient: Capability<&{NonFungibleToken.CollectionPublic}>, launchPoolId: String) {
            // create a new NFT
            let newNFT <- create NFT(
                id: MetapierLaunchpadOwnerPass.totalSupply,
                launchPoolId: launchPoolId
            )

            emit PassMinted(id: newNFT.id, launchPoolId: launchPoolId, for: recipient.address)

            // deposit it in the recipient's account using their reference
            recipient.borrow()!.deposit(token: <- newNFT)

            MetapierLaunchpadOwnerPass.totalSupply = MetapierLaunchpadOwnerPass.totalSupply + 1
        }
    }

    init() {
        self.totalSupply = 0

        self.CollectionStoragePath = /storage/MetapierLaunchpadOwnerCollection
        self.CollectionPublicPath = /public/MetapierLaunchpadOwnerCollection

        let minter <- create Minter()
        self.account.save(<- minter, to: /storage/MetapierLaunchpadOwnerMinter)

        emit ContractInitialized()
    }
}
