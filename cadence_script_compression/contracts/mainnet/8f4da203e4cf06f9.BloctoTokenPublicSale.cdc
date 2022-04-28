/*

    BloctoTokenPublicSale

    The BloctoToken Public Sale contract is used for 
    BLT token public sale. Qualified purchasers
    can purchase with tUSDT (Teleported Tether) to get
    BLTs without lockup

 */
 
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import BloctoToken from 0x0f9df91c9121c460
import TeleportedTetherToken from 0xcfdd90d4a00f7b5b

pub contract BloctoTokenPublicSale {

    /****** Sale Events ******/

    pub event NewPrice(price: UFix64)
    pub event NewPersonalCap(personalCap: UFix64)

    pub event Purchased(address: Address, amount: UFix64, ticketId: UInt64)
    pub event Distributed(address: Address, tusdtAmount: UFix64, bltAmount: UFix64)
    pub event Refunded(address: Address, amount: UFix64)

    /****** Sale Enums ******/

    pub enum PurchaseState: UInt8 {
        pub case initial
        pub case distributed
        pub case refunded
    }

    /****** Sale Resources ******/

    // BLT holder vault
    access(contract) let bltVault: @BloctoToken.Vault

    // tUSDT holder vault
    access(contract) let tusdtVault: @TeleportedTetherToken.Vault

    /// Paths for storing sale resources
    pub let SaleAdminStoragePath: StoragePath
    
    /****** Sale Variables ******/

    access(contract) var isSaleActive: Bool

    // BLT token price (tUSDT per BLT)
    access(contract) var price: UFix64

    // BLT communitu sale purchase cap (in tUSDT)
    access(contract) var personalCap: UFix64

    // All purchase records
    access(contract) var purchases: {Address: PurchaseInfo}

    // Workaround random number generator
    pub resource Random {}

    pub struct PurchaseInfo {
        // Purchaser address
        pub let address: Address

        // Purchase amount in tUSDT
        pub(set) var amount: UFix64

        // Refunded amount in tUSDT
        pub(set) var refundAmount: UFix64

        // Random ticked ID
        pub let ticketId: UInt64

        // State of the purchase
        pub(set) var state: PurchaseState

        init(
            address: Address,
            amount: UFix64,
        ) {
            // Create random resource 
            let random <- create Random()
            let ticketId = random.uuid
            destroy random

            self.address = address
            self.amount = amount
            self.refundAmount = 0.0
            self.ticketId = ticketId % 1_073_741_824 // 2^30
            self.state = PurchaseState.initial
        }
    }

    // BLT purchase method
    // User pays tUSDT and get unlocked BloctoToken
    // Note that "address" can potentially be faked, but there's no incentive doing so
    pub fun purchase(from: @TeleportedTetherToken.Vault, address: Address) {
        pre {
            self.isSaleActive: "Token sale is not active"
            self.purchases[address] == nil: "Already purchased by the same account"
            from.balance <= self.personalCap: "Purchase amount exceeds personal cap"
        }

        let amount = from.balance
        self.tusdtVault.deposit(from: <- from)

        let purchaseInfo = PurchaseInfo(address: address, amount: amount)
        self.purchases[address] = purchaseInfo

        emit Purchased(address: address, amount: amount, ticketId: purchaseInfo.ticketId)
    }

    pub fun getIsSaleActive(): Bool {
        return self.isSaleActive
    }

    // Get all purchaser addresses
    pub fun getPurchasers(): [Address] {
        return self.purchases.keys
    }

    // Get all purchase records
    pub fun getPurchases(): {Address: PurchaseInfo} {
        return self.purchases
    }

    // Get purchase record from an address
    pub fun getPurchase(address: Address): PurchaseInfo? {
        return self.purchases[address]
    }

    pub fun getBltVaultBalance(): UFix64 {
        return self.bltVault.balance
    }

    pub fun getTusdtVaultBalance(): UFix64 {
        return self.tusdtVault.balance
    }

    pub fun getPrice(): UFix64 {
        return self.price
    }

    pub fun getPersonalCap(): UFix64 {
        return self.personalCap
    }

    pub resource Admin {
        pub fun unfreeze() {
            BloctoTokenPublicSale.isSaleActive = true
        }

        pub fun freeze() {
            BloctoTokenPublicSale.isSaleActive = false
        }

        // Distribute BLT with an allocation amount
        // If user's purchase amount exceeds allocation amount, the remainder will be refunded
        pub fun distribute(address: Address, allocationAmount: UFix64) {
            pre {
                BloctoTokenPublicSale.purchases[address] != nil: "Cannot find purchase record for the address"
                BloctoTokenPublicSale.purchases[address]?.state == PurchaseState.initial: "Already distributed or refunded"
            }

            let receiverRef = getAccount(address).getCapability(BloctoToken.TokenPublicReceiverPath)
                .borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow BloctoToken receiver reference")

            let purchaseInfo = BloctoTokenPublicSale.purchases[address]
                ?? panic("Count not get purchase info for the address")

            // Make sure allocation amount does not exceed purchase amount
            assert (
                allocationAmount <= purchaseInfo.amount,
                message: "Allocation amount exceeds purchase amount"
            )

            let refundAmount = purchaseInfo.amount - allocationAmount
            let bltAmount = allocationAmount / BloctoTokenPublicSale.price
            let bltVault <- BloctoTokenPublicSale.bltVault.withdraw(amount: bltAmount)

            // Set the state of the purchase to DISTRIBUTED
            purchaseInfo.state = PurchaseState.distributed
            purchaseInfo.amount = allocationAmount
            purchaseInfo.refundAmount = refundAmount
            BloctoTokenPublicSale.purchases[address] = purchaseInfo

            // Deposit the withdrawn tokens in the recipient's receiver
            receiverRef.deposit(from: <- bltVault)

            emit Distributed(address: address, tusdtAmount: allocationAmount, bltAmount: bltAmount)

            // Refund the remaining amount
            if refundAmount > 0.0 {
                let tUSDTReceiverRef = getAccount(address).getCapability(TeleportedTetherToken.TokenPublicReceiverPath)
                    .borrow<&{FungibleToken.Receiver}>()
                    ?? panic("Could not borrow tUSDT vault receiver public reference")
                
                let tusdtVault <- BloctoTokenPublicSale.tusdtVault.withdraw(amount: refundAmount)

                tUSDTReceiverRef.deposit(from: <- tusdtVault)

                emit Refunded(address: address, amount: refundAmount)
            }
        }

        pub fun refund(address: Address) {
            pre {
                BloctoTokenPublicSale.purchases[address] != nil: "Cannot find purchase record for the address"
                BloctoTokenPublicSale.purchases[address]?.state == PurchaseState.initial: "Already distributed or refunded"
            }

            let receiverRef = getAccount(address).getCapability(TeleportedTetherToken.TokenPublicReceiverPath)
                .borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow tUSDT vault receiver public reference")

            let purchaseInfo = BloctoTokenPublicSale.purchases[address]
                ?? panic("Count not get purchase info for the address")

            let tusdtVault <- BloctoTokenPublicSale.tusdtVault.withdraw(amount: purchaseInfo.amount)

            // Set the state of the purchase to REFUNDED
            purchaseInfo.state = PurchaseState.refunded
            BloctoTokenPublicSale.purchases[address] = purchaseInfo

            receiverRef.deposit(from: <- tusdtVault)

            emit Refunded(address: address, amount: purchaseInfo.amount)
        }

        pub fun updatePrice(price: UFix64) {
            pre {
                price > 0.0: "Sale price cannot be 0"
            }

            BloctoTokenPublicSale.price = price
            emit NewPrice(price: price)
        }

        pub fun updatePersonalCap(personalCap: UFix64) {
            BloctoTokenPublicSale.personalCap = personalCap
            emit NewPersonalCap(personalCap: personalCap)
        }

        pub fun withdrawBlt(amount: UFix64): @FungibleToken.Vault {
            return <- BloctoTokenPublicSale.bltVault.withdraw(amount: amount)
        }

        pub fun withdrawTusdt(amount: UFix64): @FungibleToken.Vault {
            return <- BloctoTokenPublicSale.tusdtVault.withdraw(amount: amount)
        }

        pub fun depositBlt(from: @FungibleToken.Vault) {
            BloctoTokenPublicSale.bltVault.deposit(from: <- from)
        }

        pub fun depositTusdt(from: @FungibleToken.Vault) {
            BloctoTokenPublicSale.tusdtVault.deposit(from: <- from)
        }
    }

    init() {
        // Needs Admin to start manually
        self.isSaleActive = false

        // 1 BLT = 0.4 tUSDT
        self.price = 0.4

        // Each user can purchase at most 500 tUSDT worth of BLT
        self.personalCap = 500.0

        self.purchases = {}
        self.SaleAdminStoragePath = /storage/bloctoTokenPublicSaleAdmin

        self.bltVault <- BloctoToken.createEmptyVault() as! @BloctoToken.Vault
        self.tusdtVault <- TeleportedTetherToken.createEmptyVault() as! @TeleportedTetherToken.Vault

        let admin <- create Admin()
        self.account.save(<- admin, to: self.SaleAdminStoragePath)
    }
}
