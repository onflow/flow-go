/*

    LockedTokens implements the functionality required to manage FLOW
    buyers locked tokens from the token sale.

    Each token holder gets two accounts. One account is their locked token
    account. It will be jointly controlled by the user and the token administrator.
    The token administrator must co-sign the transfer of any locked tokens. 
    The token admin cannot interact with the account 
    without approval from the token holder, 
    except to deposit additional locked FLOW 
    or to unlock existing FLOW at each milestone in the token vesting period.
        
    The second account is the unlocked user account. This account is
    in full possesion and control of the user and they can do whatever
    they want with it. This account will store a capability that allows
    them to withdraw tokens when they become unlocked and also to
    perform staking operations with their locked tokens.

    When a user account is created, both accounts are initialized with
    their respective objects: LockedTokenManager for the shared account,
    and TokenHolder for the unlocked account. The user calls functions
    on TokenHolder to withdraw tokens from the shared account and to 
    perform staking actions with the locked tokens

 */

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import FlowIDTableStaking from 0xdee35303492e5a0b

import StakingProxy from 0x3a84eade58d45ad4

pub contract LockedTokens {

    pub event SharedAccountRegistered(address: Address)
    pub event UnlockedAccountRegistered(address: Address)

    /// Path to store the locked token manager resource 
    /// in the shared account
    pub let LockedTokenManagerStoragePath: Path

    /// Path to store the private capability for the token
    /// manager
    pub let LockedTokenManagerPrivatePath: Path

    /// Path to store the private locked token admin link
    /// in the shared account
    pub let LockedTokenAdminPrivatePath: Path

    /// Path to store the admin collection 
    /// in the admin account
    pub let LockedTokenAdminCollectionStoragePath: Path

    /// Path to store the token holder resource
    /// in the unlocked account
    pub let TokenHolderStoragePath: Path

    /// Public path to store the capability that allows
    /// reading information about a locked account
    pub let LockedAccountInfoPublicPath: Path

    /// Path that an account creator would store
    /// the resource that they use to create locked accounts
    pub let LockedAccountCreatorStoragePath: Path

    /// Path that an account creator would publish
    /// their capability for the token admin to 
    /// deposit the account creation capability
    pub let LockedAccountCreatorPublicPath: Path

    /// The TokenAdmin capability allows the token administrator to unlock tokens at each
    /// milestone in the vesting period.
    pub resource interface TokenAdmin {
        pub fun increaseUnlockLimit(delta: UFix64)
    }

    /// This token manager resource is stored in the shared account to manage access 
    /// to the locked token vault and to the staking/delegating resources.
    pub resource LockedTokenManager: FungibleToken.Receiver, FungibleToken.Provider, TokenAdmin {
    
        /// This is a reference to the default FLOW vault stored in the shared account. 
        ///
        /// All locked FLOW tokens are stored in this vault, which can be accessed in two ways:
        ///   1) Directly, in a transaction co-signed by both the token holder and token administrator
        ///   2) Indirectly via the LockedTokenManager, in a transaction signed by the token holder
        pub var vault: Capability<&FlowToken.Vault>

        /// The amount of tokens that the user can withdraw. 
        /// It is decreased when the user withdraws
        pub var unlockLimit: UFix64

        /// Optional NodeStaker resource. Will only be filled if the user
        /// signs up to be a node operator
        pub var nodeStaker: @FlowIDTableStaking.NodeStaker?

        /// Optional NodeDelegator resource. Will only be filled if the user
        /// signs up to be a delegator
        pub var nodeDelegator: @FlowIDTableStaking.NodeDelegator?

        init(vault: Capability<&FlowToken.Vault>) {
            self.vault = vault
            self.nodeStaker <- nil
            self.nodeDelegator <- nil
            self.unlockLimit = 0.0
        }

        destroy () {
            destroy self.nodeStaker
            destroy self.nodeDelegator
        }

        // FungibleToken.Receiver actions

        /// Deposits unlocked tokens to the vault
        pub fun deposit(from: @FungibleToken.Vault) {
            self.depositUnlockedTokens(from: <-from)
        }

        access(self) fun depositUnlockedTokens(from: @FungibleToken.Vault) {
            let vaultRef = self.vault.borrow()!

            let balance = from.balance

            vaultRef.deposit(from: <- from)

            self.increaseUnlockLimit(delta: balance)
        }

        // FungibleToken.Provider actions

        /// Withdraws unlocked tokens from the vault
        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            return <-self.withdrawUnlockedTokens(amount: amount)
        }

        access(self) fun withdrawUnlockedTokens(amount: UFix64): @FungibleToken.Vault {
            pre {
                self.unlockLimit >= amount: "Requested amount exceeds unlocked token limit"
            }

            post {
                self.unlockLimit == before(self.unlockLimit) - amount: "Updated unlocked token limit is incorrect"
            }

            let vaultRef = self.vault.borrow()!

            let vault <- vaultRef.withdraw(amount: amount)

            self.decreaseUnlockLimit(delta: amount)

            return <-vault
        }

        pub fun getBalance(): UFix64 {
            let vaultRef = self.vault.borrow()!
            return vaultRef.balance
        }

        access(self) fun decreaseUnlockLimit(delta: UFix64) {  
            self.unlockLimit = self.unlockLimit - delta
        }

        // LockedTokens.TokenAdmin actions

        /// Called by the admin every time a vesting release happens
        pub fun increaseUnlockLimit(delta: UFix64) {  
            self.unlockLimit = self.unlockLimit + delta
        }

        // LockedTokens.TokenHolder actions

        /// Registers a new node operator with the Flow Staking contract
        /// and commits an initial amount of locked tokens to stake
        pub fun registerNode(nodeInfo: StakingProxy.NodeInfo, amount: UFix64) {
            let vaultRef = self.vault.borrow()!

            let tokens <- vaultRef.withdraw(amount: amount)

            self.nodeStaker <-! FlowIDTableStaking.addNodeRecord(id: nodeInfo.id, role: nodeInfo.role, networkingAddress: nodeInfo.networkingAddress, networkingKey: nodeInfo.networkingKey, stakingKey: nodeInfo.stakingKey, tokensCommitted: <-tokens)
        }

        /// Registers a new Delegator with the Flow Staking contract
        /// the caller has to specify the ID of the node operator
        /// they are delegating to
        pub fun registerDelegator(nodeID: String) {
            self.nodeDelegator <-! FlowIDTableStaking.registerNewDelegator(nodeID: nodeID)
        }
    }

    /// This interfaces allows anybody to read information about the locked account.
    pub resource interface LockedAccountInfo {
        pub fun getLockedAccountAddress(): Address
        pub fun getLockedAccountBalance(): UFix64
        pub fun getUnlockLimit(): UFix64
    }

    /// Stored in Holder unlocked account
    pub resource TokenHolder: FungibleToken.Receiver, FungibleToken.Provider, LockedAccountInfo {

        /// The address of the shared (locked) account.
        pub var address: Address

        /// Capability that is used to access the LockedTokenManager
        /// in the shared account
        access(self) var tokenManager: Capability<&LockedTokenManager>

        /// Used to perform staking actions if the user has signed up
        /// as a node operator
        access(self) var nodeStakerProxy: LockedNodeStakerProxy?

        /// Used to perform delegating actions if the user has signed up
        /// as a delegator
        access(self) var nodeDelegatorProxy: LockedNodeDelegatorProxy?

        init(lockedAddress: Address, tokenManager: Capability<&LockedTokenManager>) {
            pre {
                tokenManager.borrow() != nil: "Must pass a LockedTokenManager capability"
            }

            self.address = lockedAddress
            self.tokenManager = tokenManager
            self.nodeStakerProxy = nil
            self.nodeDelegatorProxy = nil
        }

        /// Utility function to borrow a reference to the LockedTokenManager object
        access(self) fun borrowTokenManager(): &LockedTokenManager {
            return self.tokenManager.borrow()!
        }

        // LockedAccountInfo actions

        /// Returns the locked account address for this token holder.
        pub fun getLockedAccountAddress(): Address {
            return self.address
        }

        /// Returns the locked account balance for this token holder.
        pub fun getLockedAccountBalance(): UFix64 {
            return self.borrowTokenManager().getBalance()
        }

        // Returns the unlocked limit for this token holder.
        pub fun getUnlockLimit(): UFix64 {
            return self.borrowTokenManager().unlockLimit
        }

        /// Deposits tokens in the locked vault, which marks them as 
        /// unlocked and available to withdraw
        pub fun deposit(from: @FungibleToken.Vault) {
            self.borrowTokenManager().deposit(from: <-from)
        }

        // FungibleToken.Provider actions

        /// Withdraws tokens from the locked vault. This will only succeed
        /// if the withdraw amount is less than or equal to the limit
        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            return <- self.borrowTokenManager().withdraw(amount: amount)
        }

        /// The user calls this function if they want to register as a node operator
        /// They have to provide all the info for their node
        pub fun createNodeStaker(nodeInfo: StakingProxy.NodeInfo, amount: UFix64) {
            pre {
                self.nodeStakerProxy == nil && self.nodeDelegatorProxy == nil: "Already initialized"
            }

            self.borrowTokenManager().registerNode(nodeInfo: nodeInfo, amount: amount)

            // Create a new staker proxy that can be accessed in transactions
            self.nodeStakerProxy = LockedNodeStakerProxy(tokenManager: self.tokenManager)
        }

        /// The user calls this function if they want to register as a node operator
        /// They have to provide the node ID for the node they want to delegate to
        pub fun createNodeDelegator(nodeID: String) {
            pre {
                self.nodeStakerProxy == nil && self.nodeDelegatorProxy == nil: "Already initialized"
            }

            self.borrowTokenManager().registerDelegator(nodeID: nodeID)

            // create a new delegator proxy that can be accessed in transactions
            self.nodeDelegatorProxy = LockedNodeDelegatorProxy(tokenManager: self.tokenManager)
        }

        /// Borrow a "reference" to the staking object which allows the caller
        /// to perform all staking actions with locked tokens.
        pub fun borrowStaker(): LockedNodeStakerProxy {
            pre {
                self.nodeStakerProxy != nil:
                    "The NodeStakerProxy doesn't exist!"
            }
            return self.nodeStakerProxy!
        }

        /// Borrow a "reference" to the delegating object which allows the caller
        /// to perform all delegating actions with locked tokens.
        pub fun borrowDelegator(): LockedNodeDelegatorProxy {
            pre {
                self.nodeDelegatorProxy != nil:
                    "The NodeDelegatorProxy doesn't exist!"
            }
            return self.nodeDelegatorProxy!
        }
    }

    /// Used to perform staking actions
    pub struct LockedNodeStakerProxy: StakingProxy.NodeStakerProxy {

        access(self) var tokenManager: Capability<&LockedTokenManager>

        init(tokenManager: Capability<&LockedTokenManager>) {
            pre {
                tokenManager.borrow() != nil: "Invalid token manager capability"
            }
            self.tokenManager = tokenManager
        }

        /// Stakes new locked tokens
        pub fun stakeNewTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            let vaultRef = tokenManagerRef.vault.borrow()!

            tokenManagerRef.nodeStaker?.stakeNewTokens(<-vaultRef.withdraw(amount: amount))
        }

        /// Stakes unstaked tokens from the staking contract
        pub fun stakeUnstakedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeStaker?.stakeUnstakedTokens(amount: amount)
        }

        /// Stakes rewarded tokens. Rewarded tokens are freely withdrawable
        /// so if they are staked, the withdraw limit should be increased
        /// because staked tokens are effectively treated as locked tokens
        pub fun stakeRewardedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeStaker?.stakeRewardedTokens(amount: amount)

            tokenManagerRef.increaseUnlockLimit(delta: amount)
        }

        /// Requests unstaking for the node
        pub fun requestUnstaking(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeStaker?.requestUnstaking(amount: amount)
        }

        /// Requests to unstake all of the node's tokens and all of
        /// the tokens that have been delegated to the node
        pub fun unstakeAll() {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeStaker?.unstakeAll()
        }

        /// Withdraw the unstaked tokens back to 
        /// the locked token vault. This does not increase the withdraw
        /// limit because staked/unstaked tokens are considered to still
        /// be locked in terms of the vesting schedule
        pub fun withdrawUnstakedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            let vaultRef = tokenManagerRef.vault.borrow()!

            let withdrawnTokens <- tokenManagerRef.nodeStaker?.withdrawUnstakedTokens(amount: amount)!

            vaultRef.deposit(from: <-withdrawnTokens)
        }

        /// Withdraw reward tokens to the locked vault, 
        /// which increases the withdraw limit
        pub fun withdrawRewardedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.deposit(from: <-tokenManagerRef.nodeStaker?.withdrawRewardedTokens(amount: amount)!)
        }
    }

    /// Used to perform delegating actions in transactions
    pub struct LockedNodeDelegatorProxy: StakingProxy.NodeDelegatorProxy {

        access(self) var tokenManager: Capability<&LockedTokenManager>

        init(tokenManager: Capability<&LockedTokenManager>) {
            pre {
                tokenManager.borrow() != nil: "Invalid LockedTokenManager capability"
            }
            self.tokenManager = tokenManager
        }

        /// delegates tokens from the locked token vault
        pub fun delegateNewTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            let vaultRef = tokenManagerRef.vault.borrow()!

            tokenManagerRef.nodeDelegator?.delegateNewTokens(from: <-vaultRef.withdraw(amount: amount))
        }

        /// Delegate tokens from the unstaked staking bucket
        pub fun delegateUnstakedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeDelegator?.delegateUnstakedTokens(amount: amount)
        }

        /// Delegate rewarded tokens. Increases the unlock limit
        /// because these are freely withdrawable
        pub fun delegateRewardedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeDelegator?.delegateRewardedTokens(amount: amount)

            tokenManagerRef.increaseUnlockLimit(delta: amount)
        }

        /// Request to unstake tokens
        pub fun requestUnstaking(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.nodeDelegator?.requestUnstaking(amount: amount)
        }

        /// withdraw unstaked tokens back to the locked vault
        /// This does not increase the withdraw limit
        pub fun withdrawUnstakedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            let vaultRef = tokenManagerRef.vault.borrow()!

            vaultRef.deposit(from: <-tokenManagerRef.nodeDelegator?.withdrawUnstakedTokens(amount: amount)!)
        }

        /// Withdraw rewarded tokens back to the locked vault,
        /// which increases the withdraw limit because these 
        /// are considered unstaked in terms of the vesting schedule
        pub fun withdrawRewardedTokens(amount: UFix64) {
            let tokenManagerRef = self.tokenManager.borrow()!

            tokenManagerRef.deposit(from: <-tokenManagerRef.nodeDelegator?.withdrawRewardedTokens(amount: amount)!)
        }
    }

    pub resource interface AddAccount {
        pub fun addAccount(
            sharedAccountAddress: Address, 
            unlockedAccountAddress: Address,
            tokenAdmin: Capability<&LockedTokenManager>)
    }

    /// Resource that the Dapper Labs token admin
    /// stores in their account to manage the vesting schedule
    /// for all the token holders
    pub resource TokenAdminCollection: AddAccount {
        
        /// Mapping of account addresses to LockedTokenManager capabilities
        access(self) var accounts: {Address: Capability<&LockedTokenManager>}

        init() {
            self.accounts = {}
        }

        /// Add a new account's locked token manager capability
        /// to the record
        pub fun addAccount(
            sharedAccountAddress: Address, 
            unlockedAccountAddress: Address,
            tokenAdmin: Capability<&LockedTokenManager>) 
        {
            self.accounts[sharedAccountAddress] = tokenAdmin
            emit SharedAccountRegistered(address: sharedAccountAddress)
            emit UnlockedAccountRegistered(address: unlockedAccountAddress)
        }

        /// Get an accounts capability
        pub fun getAccount(address: Address): Capability<&LockedTokenManager> {
            return self.accounts[address]!
        }
    }

    pub resource interface LockedAccountCreatorPublic {
        pub fun addCapability(cap: Capability<&TokenAdminCollection>)
    }

    // account creators store this resource in their account
    // in order to be able to register accounts who have locked tokens
    pub resource LockedAccountCreator: LockedAccountCreatorPublic, AddAccount {

        access(self) var addAccountCapability: Capability<&TokenAdminCollection>?

        init() {
            self.addAccountCapability = nil
        }

        pub fun addCapability(cap: Capability<&TokenAdminCollection>) {
            self.addAccountCapability = cap
        }

        pub fun addAccount(sharedAccountAddress: Address, 
                           unlockedAccountAddress: Address,
                           tokenAdmin: Capability<&LockedTokenManager>) {
            
            let adminRef = self.addAccountCapability!.borrow()!

            adminRef.addAccount(sharedAccountAddress: sharedAccountAddress, 
                           unlockedAccountAddress: unlockedAccountAddress,
                           tokenAdmin: tokenAdmin)
        }
    }

    /// Public function to create a new token admin collection
    pub fun createTokenAdminCollection(): @TokenAdminCollection {
        return <- create TokenAdminCollection()
    }

    /// Public function to create a new Locked Token Manager
    /// every time a new user account is created
    pub fun createLockedTokenManager(vault: Capability<&FlowToken.Vault>): @LockedTokenManager {
        return <- create LockedTokenManager(vault: vault)
    }

    // Creates a new TokenHolder resource for this LockedTokenManager
    /// that the user can store in their unlocked account.
    pub fun createTokenHolder(lockedAddress: Address, tokenManager: Capability<&LockedTokenManager>): @TokenHolder {
        return <- create TokenHolder(lockedAddress: lockedAddress, tokenManager: tokenManager)
    }

    pub fun createLockedAccountCreator(): @LockedAccountCreator {
        return <-create LockedAccountCreator()
    }

    init() {
        self.LockedTokenManagerStoragePath = /storage/lockedTokenManager
        self.LockedTokenManagerPrivatePath = /private/lockedTokenManager

        self.LockedTokenAdminPrivatePath = /private/lockedTokenAdmin
        self.LockedTokenAdminCollectionStoragePath = /storage/lockedTokenAdminCollection

        self.TokenHolderStoragePath = /storage/flowTokenHolder
        self.LockedAccountInfoPublicPath = /public/lockedAccountInfo

        self.LockedAccountCreatorStoragePath = /storage/lockedAccountCreator
        self.LockedAccountCreatorPublicPath = /public/lockedAccountCreator
    }
}
