import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61

pub contract NFTLendingPlace {

    // Event emitted when a new NFT is listed as collateral
    pub event ForLend(address: Address, kind: Type, id: UInt64, uuid: UInt64, baseAmount: UFix64, interest: UFix64, duration: UFix64)

    // Event emitted when the borrowing amount of NFT changed
    pub event BaseAmountChanged(id: UInt64, newBaseAmount: UFix64)

    // Event emitted when the borrowing fee of NFT changed
    pub event InterestChanged(id: UInt64, newInterest: UFix64)

    // Event emitted when the duration of NFT changed
    pub event DurationChanged(id: UInt64, newDuration: UFix64)
    
    // Event emitted when the lender lends out money
    pub event LendOut(address: Address, kind: Type?, uuid: UInt64, baseAmount: UFix64, interest: UFix64, beginningTime: UFix64, duration: UFix64)

    // Event emitted when the borrower repays
    pub event Repay(kind: Type?, uuid: UInt64, repayAmount: UFix64, time: UFix64)

    // Event emitted when the lender forces redeem
    pub event ForcedRedeem(kind: Type?, uuid: UInt64, time: UFix64)

    // Event emitted when the NFT owner withdraws NFT from lending resource
    pub event CaseWithdrawn(uuid: UInt64)

    // Interface for users to publish their lending collection, which only exposes public methods
    pub resource interface LendingManager {
        pub fun withdraw(uuid: UInt64): @NonFungibleToken.NFT
        pub fun listForLending(owner: Address, token: @NonFungibleToken.NFT, baseAmount: UFix64, interest: UFix64, duration: UFix64)
        pub fun repay(uuid: UInt64, repayAmount: @FlowToken.Vault): @NonFungibleToken.NFT
    }

    pub resource interface LendingPublic {
        pub fun lendOut(uuid: UInt64, recipient: Address, lendAmount: @FlowToken.Vault, ticket: &LenderTicket)
        pub fun forcedRedeem(uuid: UInt64, lendticket: &LenderTicket): @NonFungibleToken.NFT
        pub fun idBaseAmounts(uuid: UInt64): UFix64?
        pub fun idInterests(uuid: UInt64): UFix64?
        pub fun idDuration(uuid: UInt64): UFix64?
        pub fun idLenders(uuid: UInt64): Address?
        pub fun idKinds(uuid: UInt64): Type?
        pub fun getIDs(): [UInt64]
    }

    // LendingCollection
    //
    // The NFT collection object where users can put their NFT in as collateral, 
    // or spend fungible tokens to lend others' NFT
    pub resource LendingCollection: LendingPublic, LendingManager {

        // Dictionary of the NFTs that user listed for lending
        access(self) var forLend: @{UInt64: NonFungibleToken.NFT}

        // Dictionary of the prices for each NFT, sorted by uuid
        access(self) var baseAmounts: {UInt64: UFix64}
        // Dictionary of the interests for each NFT, sorted by uuid
        access(self) var interests: {UInt64: UFix64} 
        // Dictionary of the duration for each NFT, sorted by uuid
        access(self) var duration: {UInt64: UFix64}
        // Dictionary of the beginningTime for each NFT, sorted by uuid
        access(self) var beginningTime: {UInt64: UFix64}
        // Dictionary of the lenders for each NFT, sorted by uuid
        access(self) var lenders: {UInt64: Address}
        // Dictionary of the type for each NFT, sorted by uuid
        access(self) var kinds: {UInt64: Type}

        // The owner's fungible token vault for this lending
        // When a user lends token, this resource can deposit that token into his account
        access(account) let ownerVault: Capability<&FlowToken.Vault{FungibleToken.Receiver}>

        init (vault: Capability<&FlowToken.Vault{FungibleToken.Receiver}>) {
            self.forLend <- {}

            self.ownerVault = vault
            self.baseAmounts = {}
            self.interests = {}
            self.beginningTime = {}
            self.duration = {} // ex: 5000 seconds

            self.lenders = {}
            self.kinds = {}
        }

        // listForLending lists an NFT as collateral
        pub fun listForLending(owner: Address, token: @NonFungibleToken.NFT, baseAmount: UFix64, interest: UFix64, duration: UFix64) {
            let uuid = token.uuid
            let type = token.getType()

            // store the price in the price array
            self.baseAmounts[uuid] = baseAmount

            self.interests[uuid] = interest

            self.duration[uuid] = duration

            self.kinds[uuid] = type

            emit ForLend(address: owner, kind: type, id: token.id, uuid: uuid, baseAmount: baseAmount, interest: interest, duration: duration)

            // put the NFT into the ForLend dictionary
            let oldToken <- self.forLend[uuid] <- token
            destroy oldToken
        }

        // withdraw gives NFT owners a chance to unlist NFT as collateral
        pub fun withdraw(uuid: UInt64): @NonFungibleToken.NFT {
            pre { self.lenders[uuid] == nil : "The NFT is being used to lend money"
                  self.baseAmounts[uuid] != nil : "baseAmount hasn't been set, and the NFT hasn't been listed as colleteral"
            }

            self.baseAmounts.remove(key: uuid)

            self.duration.remove(key: uuid)
            
            self.interests.remove(key: uuid)

            self.lenders.remove(key: uuid)

            self.kinds.remove(key: uuid)

            emit CaseWithdrawn(uuid: uuid)

            // remove and return the token
            let token <- self.forLend.remove(key: uuid) ?? panic("Can't find the NFT in the forLend dictionary")
            return <-token
        }

        // changeBaseAmount changes the currently lending token amount
        pub fun changeBaseAmount(uuid: UInt64, newBaseAmount: UFix64) {
            pre { self.lenders[uuid] == nil : "This NFT is being used to lend money"
                  self.baseAmounts[uuid] != nil : "The baseAmount should be set first"
            }

            self.baseAmounts[uuid] = newBaseAmount

            emit BaseAmountChanged(id: uuid, newBaseAmount: newBaseAmount)
        }

        pub fun changeInterest(uuid: UInt64, newInterest: UFix64) {
            pre { self.lenders[uuid] == nil : "This NFT is being used to lend money"
                  self.interests[uuid] != nil : "The interests should be set first"
            }

            self.interests[uuid] = newInterest

            emit InterestChanged(id: uuid, newInterest: newInterest)
        }

        pub fun changeExpiredBlock(uuid: UInt64, newDuration: UFix64) {
            pre { self.lenders[uuid] == nil : "This NFT is being used to lend money"
                  self.duration[uuid] != nil : "The duration should be set first"
            }

            self.duration[uuid] = newDuration

            emit DurationChanged(id: uuid, newDuration: newDuration)
        }

        // lendOut lets a user lend tokens to the borrower
        pub fun lendOut(uuid: UInt64, recipient: Address, lendAmount: @FlowToken.Vault, ticket: &LenderTicket) {
            pre {
                self.forLend[uuid] != nil:
                    "No token matching this uuid for lending!"

                lendAmount.balance >= (self.baseAmounts[uuid] ?? 0.0):
                    "Not enough tokens to lend!"

                self.lenders[uuid] == nil : "This NFT is being used to lend money"

                self.beginningTime[uuid] == nil : "must no beginning time for this NFT lending"
            }


            self.beginningTime[uuid] = getCurrentBlock().timestamp

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            
            // deposit the purchasing tokens into the owners vault
            vaultRef.deposit(from: <- lendAmount)

            self.lenders[uuid] = recipient

            ticket.changeticket(uuid: uuid, value: true)

            emit LendOut(address: recipient, kind: self.kinds[uuid], uuid: uuid, baseAmount: self.baseAmounts[uuid]!, interest: self.interests[uuid]!, beginningTime: self.beginningTime[uuid]!, duration: self.duration[uuid]!)
        }

        // Repay lets the borrower repays token to lender
        pub fun repay(uuid: UInt64, repayAmount: @FlowToken.Vault): @NonFungibleToken.NFT {
            pre {
                self.forLend[uuid] != nil:
                    "No token matching this ID for lending!"

                repayAmount.balance >= (self.baseAmounts[uuid] ?? 0.0) + (self.interests[uuid] ?? 0.0) :
                    "Not enough tokens to repay!"

                self.lenders[uuid] != nil : "There is no lender now"

                self.beginningTime[uuid] != nil : "The lending has not started yet"

                (self.duration[uuid] ?? 0.0 as UFix64) + (self.beginningTime[uuid] ?? 0.0 as UFix64) >= getCurrentBlock().timestamp : 
                    "Must lower than the current block's timestamp" 
            }

            // pay
            let _repayAmount = repayAmount.balance
            if let vaultRef = getAccount(self.lenders[uuid]!).getCapability(/public/flowTokenReceiver)
                      .borrow<&FlowToken.Vault{FungibleToken.Receiver}>() {
                vaultRef.deposit(from: <-repayAmount)
            } else {
                let vaultRef = getAccount(0xd5613003fe383df9).getCapability(/public/flowTokenReceiver)
                      .borrow<&FlowToken.Vault{FungibleToken.Receiver}>() ?? panic("Could not borrow reference to admin token vault")
                vaultRef.deposit(from: <-repayAmount)
            }
            
            self.lenders[uuid] = nil
            self.beginningTime[uuid] = nil
            

            emit Repay(kind:self.kinds[uuid],uuid: uuid, repayAmount: _repayAmount, time: getCurrentBlock().timestamp)

            return <- self.withdraw(uuid: uuid)
        }

        // forceRedeem lets the lender force redeem the NFT from borrower when expiration
        pub fun forcedRedeem(uuid: UInt64, lendticket: &LenderTicket): @NonFungibleToken.NFT {
            pre {
                lendticket.owner?.address == self.lenders[uuid] : "The lender and the ticket owner are not the same"

                self.forLend[uuid] != nil:
                    "No token matching this uuid for lending!"

                self.lenders[uuid] != nil : "There is no lender now"

                self.beginningTime[uuid] != nil : "The lending has not started yet"
                
                lendticket.ticket[uuid] == true : "lendticket of the uuid is not true"

                (self.duration[uuid] ?? 0.0 as UFix64) + (self.beginningTime[uuid] ?? 0.0 as UFix64) < getCurrentBlock().timestamp : 
                    "Must higher than the current block's timestamp"
            }

            emit ForcedRedeem(kind: self.kinds[uuid], uuid: uuid, time: getCurrentBlock().timestamp)

            self.lenders[uuid] = nil

            self.beginningTime[uuid] = nil

            lendticket.ticket.remove(key: uuid)

            return <- self.withdraw(uuid: uuid)
        }

        pub fun idBaseAmounts(uuid: UInt64): UFix64? {
            return self.baseAmounts[uuid]
        }

        pub fun idInterests(uuid: UInt64): UFix64? {
            return self.interests[uuid]
        }

        pub fun idDuration(uuid: UInt64): UFix64? {
            return self.duration[uuid]
        }

        pub fun idLenders(uuid: UInt64): Address? {
            return self.lenders[uuid]
        }

        pub fun idKinds(uuid: UInt64): Type? {
            return self.kinds[uuid]
        }

        // getIDs returns collateral's token ID as array
        pub fun getIDs(): [UInt64] {
            return self.forLend.keys
        }

        destroy() {
            destroy self.forLend
        }
    }

    // LenderTicket is the proof of lander 
    pub resource LenderTicket {
        pub var ticket: {UInt64: Bool}
        init () {
            self.ticket = {}
        }
        access(contract) fun changeticket(uuid: UInt64, value: Bool) {
            self.ticket[uuid] = value
        }
    }
    
    pub fun createLenderTicket(): @LenderTicket {
        return <- create LenderTicket()
    }

    // create LendingCollection returns a new collection resource to the caller
    pub fun createLendingCollection(ownerVault: Capability<&FlowToken.Vault{FungibleToken.Receiver}>): @LendingCollection {
        return <- create LendingCollection(vault: ownerVault)
    }
}