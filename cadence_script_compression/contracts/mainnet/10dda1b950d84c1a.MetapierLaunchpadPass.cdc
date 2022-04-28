import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

/**

A Metapier launchpad pass is designed to help with identity verification so 
that we know funds are withdrawn from a whitelisted account, and we can also
ensure launch tokens are distributed to the same account.

Since launch pool will perform the whitelist check, anyone can mint a pass
to try to participate.

 */
pub contract MetapierLaunchpadPass: NonFungibleToken {

    pub event ContractInitialized()

    pub event Withdraw(id: UInt64, from: Address?)

    pub event Deposit(id: UInt64, to: Address?)

    pub event PassMinted(id: UInt64, for: Address)

    pub var totalSupply: UInt64
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    pub resource interface PublicPass {
        // the owner of the collection that the pass is initially deposited into
        pub let originalOwner: Address

        // the type of the funds vault used by this pass
        pub let fundsType: Type

        // the type of the launch token vault used by this pass
        pub let launchTokenType: Type

        pub fun getFundsBalance(): UFix64
        pub fun getLaunchTokenBalance(): UFix64
        pub fun depositFunds(vault: @FungibleToken.Vault)
        pub fun depositLaunchToken(vault: @FungibleToken.Vault)
    }

    pub resource interface PrivatePass {
        pub fun withdrawFunds(amount: UFix64): @FungibleToken.Vault
        pub fun withdrawLaunchToken(amount: UFix64): @FungibleToken.Vault
    }

    pub resource NFT: NonFungibleToken.INFT, PublicPass, PrivatePass {
        pub let id: UInt64

        pub let originalOwner: Address

        pub let fundsType: Type

        pub let launchTokenType: Type

        access(self) let fundsVault: @FungibleToken.Vault

        access(self) let launchTokenVault: @FungibleToken.Vault

        init(
            initID: UInt64,
            originalOwner: Address,
            fundsVault: @FungibleToken.Vault,
            launchTokenVault: @FungibleToken.Vault
        ) {
            self.id = initID
            self.originalOwner = originalOwner

            self.fundsType = fundsVault.getType()
            self.fundsVault <- fundsVault

            self.launchTokenType = launchTokenVault.getType()
            self.launchTokenVault <- launchTokenVault
        }

        pub fun getFundsBalance(): UFix64 {
            return self.fundsVault.balance
        }

        pub fun getLaunchTokenBalance(): UFix64 {
            return self.launchTokenVault.balance
        }

        pub fun depositFunds(vault: @FungibleToken.Vault) {
            self.fundsVault.deposit(from: <- vault)
        }

        pub fun depositLaunchToken(vault: @FungibleToken.Vault) {
            self.launchTokenVault.deposit(from: <- vault)
        }

        pub fun withdrawFunds(amount: UFix64): @FungibleToken.Vault {
            return <- self.fundsVault.withdraw(amount: amount)
        }

        pub fun withdrawLaunchToken(amount: UFix64): @FungibleToken.Vault {
            return <- self.launchTokenVault.withdraw(amount: amount)
        }

        destroy() {
            destroy self.fundsVault
            destroy self.launchTokenVault
        }
    }

    pub resource interface CollectionPublic {
        pub fun idExists(id: UInt64): Bool
        pub fun borrowPublicPass(id: UInt64): 
            &MetapierLaunchpadPass.NFT{MetapierLaunchpadPass.PublicPass, NonFungibleToken.INFT}
    }

    pub resource interface CollectionPrivate {
        pub fun borrowPrivatePass(id: UInt64): &MetapierLaunchpadPass.NFT
    }

    pub resource Collection:
        NonFungibleToken.Provider,
        NonFungibleToken.Receiver,
        NonFungibleToken.CollectionPublic,
        CollectionPublic,
        CollectionPrivate
    {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            // make sure the token has the right type
            let token <- token as! @MetapierLaunchpadPass.NFT
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

        pub fun borrowPublicPass(id: UInt64): 
            &MetapierLaunchpadPass.NFT{MetapierLaunchpadPass.PublicPass, NonFungibleToken.INFT}
        {
            let passRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let intermediateRef = passRef as! auth &MetapierLaunchpadPass.NFT

            return intermediateRef as &MetapierLaunchpadPass.NFT{MetapierLaunchpadPass.PublicPass, NonFungibleToken.INFT}
        }

        pub fun borrowPrivatePass(id: UInt64): &MetapierLaunchpadPass.NFT {
            let passRef = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return passRef as! &MetapierLaunchpadPass.NFT
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    // anyone can create a new pass to try participating a launch pool
    pub fun mintNewPass(
            recipient: Capability<&{NonFungibleToken.CollectionPublic}>,
            fundsVault: @FungibleToken.Vault,
            launchTokenVault: @FungibleToken.Vault
    ): UInt64 {
        let newPass <- create NFT(
            initID: MetapierLaunchpadPass.totalSupply, // id never repeats
            originalOwner: recipient.address,
            fundsVault: <- fundsVault,
            launchTokenVault: <- launchTokenVault
        )
        let newPassId = newPass.id

        MetapierLaunchpadPass.totalSupply = MetapierLaunchpadPass.totalSupply + 1

        emit PassMinted(id: newPassId, for: recipient.address)

        // save this pass into the recipient's collection
        recipient.borrow()!.deposit(token: <- newPass)

        return newPassId
    }

    init() {
        self.totalSupply = 0

        self.CollectionStoragePath = /storage/MetapierLaunchpadPassCollection
        self.CollectionPublicPath = /public/MetapierLaunchpadPassCollection

        emit ContractInitialized()
    }
}
 