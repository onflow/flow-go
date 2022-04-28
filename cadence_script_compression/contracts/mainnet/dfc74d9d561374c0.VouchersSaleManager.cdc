/*
    A contract that manages the creation and sale of Goated Goat and pack vouchers

    A manager resource exists to allow modifications to the parameters of the public
    sale and have ability to mint editions themself.
*/

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import NonFungibleToken from 0x1d7e57aa55817448
import GoatedGoatsVouchers from 0xdfc74d9d561374c0
import TraitPacksVouchers from 0xdfc74d9d561374c0
import VouchersSaleManagerHelper from 0xdfc74d9d561374c0

pub contract VouchersSaleManager {
    // -----------------------------------------------------------------------
    //  Events
    // -----------------------------------------------------------------------
    // Emitted when the contract is initialized
    pub event ContractInitialized()

    // Emitted when a goated goat collection has had metadata updated
    pub event UpdateGoatedGoatsVoucherCollectionMetadata()
    // Emitted when an edition within the goated goat collection has had
    // it's metadata updated
    pub event UpdateGoatedGoatsVoucherEditionMetadata(id: UInt64)

    // Emitted when a trait pack voucher collection has had metadata updated
    pub event UpdateTraitPacksVoucherCollectionMetadata()
    // Emitted when an edition within a voucher has had it's metadata updated
    pub event UpdateTraitPacksVoucherEditionMetadata(id: UInt64)

    // Emitted when an admin has initiated a mint of a goat and pack voucher
    pub event AdminMint(id: UInt64)
    // Emitted when someone has purchased a goat and pack voucher
    pub event PublicMint(id: UInt64)

    // Emitted when an admin has initiated a goat voucher
    pub event AdminGoatVoucherMint(id: UInt64)
    // Emitted when someone has purchased a goat voucher
    pub event PublicGoatVoucherMint(id: UInt64)

    // Emitted when an admin has initiated a pack voucher
    pub event AdminPackVoucherMint(id: UInt64)
    // Emitted when someone has purchased a pack voucher
    pub event PublicPackVoucherMint(id: UInt64)

    // Emitted when any info about sale logistics has been modified
    pub event UpdateSaleInfo(saleStartTime: UFix64, salePrice: UFix64, maxQuantityPerMint: UInt64)

    // Emitted when any info about sale logistics has been modified
    pub event UpdateGoatVoucherSaleInfo(saleStartTime: UFix64, salePrice: UFix64, maxQuantityPerMint: UInt64)

    // Emitted when any info about sale logistics has been modified
    pub event UpdatePackVoucherSaleInfo(saleStartTime: UFix64, salePrice: UFix64, maxQuantityPerMint: UInt64)

    // Emitted when the receiver of public sale payments has been updated
    pub event UpdatePaymentReceiver(address: Address)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------
    pub let ManagerStoragePath: StoragePath

    // -----------------------------------------------------------------------
    // VouchersSaleManager fields
    // -----------------------------------------------------------------------
    access(self) let mintedEditions: {UInt64: Bool}
    access(self) var sequentialMintMin: UInt64
    access(contract) var paymentReceiver: Capability<&{FungibleToken.Receiver}>

    pub var maxSupply: UInt64
    pub var totalSupply: UInt64
    pub var maxQuantityPerMint: UInt64
    pub var saleStartTime: UFix64
    pub var salePrice: UFix64

    // -----------------------------------------------------------------------
    // Manager resource
    // -----------------------------------------------------------------------
    pub resource Manager {

        pub fun updateGoatVoucherSaleInfo(quantityPerMint: UInt64, price: UFix64, startTime: UFix64) {
            VouchersSaleManagerHelper.updateGoatVoucherSale(quantityPerMint: quantityPerMint, price: price, startTime: startTime)
            emit UpdateGoatVoucherSaleInfo(
                saleStartTime: startTime,
                salePrice: price,
                maxQuantityPerMint: quantityPerMint
            )
        }

        pub fun updatePackVoucherSaleInfo(quantityPerMint: UInt64, price: UFix64, startTime: UFix64) {
            VouchersSaleManagerHelper.updatePackVoucherSale(quantityPerMint: quantityPerMint, price: price, startTime: startTime)
            emit UpdatePackVoucherSaleInfo(
                saleStartTime: startTime,
                salePrice: price,
                maxQuantityPerMint: quantityPerMint
            )
        }

        pub fun updateGoatedGoatsCollectionMetadata(metadata: {String: String}) {
            GoatedGoatsVouchers.setCollectionMetadata(metadata: metadata)
            emit UpdateGoatedGoatsVoucherCollectionMetadata()
        }

        pub fun updateGoatedGoatsEditionMetadata(editionNumber: UInt64, metadata: {String: String}) {
            GoatedGoatsVouchers.setEditionMetadata(editionNumber: editionNumber, metadata: metadata)
            emit UpdateGoatedGoatsVoucherEditionMetadata(id: editionNumber)
        }

        pub fun updateTraitPacksCollectionMetadata(metadata: {String: String}) {
            TraitPacksVouchers.setCollectionMetadata(metadata: metadata)
            emit UpdateTraitPacksVoucherCollectionMetadata()
        }
        
        pub fun updateTraitsPacksEditionMetadata(editionNumber: UInt64, metadata: {String: String}) {
            TraitPacksVouchers.setEditionMetadata(editionNumber: editionNumber, metadata: metadata)
            emit UpdateTraitPacksVoucherEditionMetadata(id: editionNumber)
        }

        pub fun setPaymentReceiver(paymentReceiver: Capability<&{FungibleToken.Receiver}>) {
            VouchersSaleManager.paymentReceiver = paymentReceiver
            emit UpdatePaymentReceiver(address: paymentReceiver.address)
        }

        pub fun mintGoatVoucherAtEdition(edition: UInt64): @NonFungibleToken.NFT {
            emit AdminGoatVoucherMint(id: edition)
            return <-VouchersSaleManager.mintGoatVoucher(edition: edition)
        }

        pub fun mintPackVoucherAtEdition(edition: UInt64): @NonFungibleToken.NFT {
            emit AdminPackVoucherMint(id: edition)
            return <-VouchersSaleManager.mintPackVoucher(edition: edition)
        }


        // -----------------------------------------------------------------------
        // Deprecated Functions
        // -----------------------------------------------------------------------
        // DEPRECATED - Use voucher specific updates instead
        pub fun updateMaxQuantityPerMint(_ amount: UInt64) {
            VouchersSaleManager.maxQuantityPerMint = amount
            emit UpdateSaleInfo(
                saleStartTime: VouchersSaleManager.saleStartTime,
                salePrice: VouchersSaleManager.salePrice,
                maxQuantityPerMint: VouchersSaleManager.maxQuantityPerMint
            )
        }
        // DEPRECATED - Use voucher specific updates instead
        pub fun updatePrice(_ price: UFix64) {
            VouchersSaleManager.salePrice = price
            emit UpdateSaleInfo(
                saleStartTime: VouchersSaleManager.saleStartTime,
                salePrice: VouchersSaleManager.salePrice,
                maxQuantityPerMint: VouchersSaleManager.maxQuantityPerMint
            )
        }
        // DEPRECATED - Use voucher specific updates instead
        pub fun updateSaleStartTime(_ saleStartTime: UFix64) {
            VouchersSaleManager.saleStartTime = saleStartTime
            emit UpdateSaleInfo(
                saleStartTime: VouchersSaleManager.saleStartTime,
                salePrice: VouchersSaleManager.salePrice,
                maxQuantityPerMint: VouchersSaleManager.maxQuantityPerMint
            )
        }
        // DEPRECATED - Use voucher specific updates instead
        pub fun mintVoucherAtEdition(edition: UInt64): @[NonFungibleToken.NFT] {
            emit AdminMint(id: edition)
            return <-VouchersSaleManager.mintVouchers(edition: edition)
        }
    }

    // Mint a voucher for goated goats
    access(contract) fun mintGoatVoucher(edition: UInt64): @NonFungibleToken.NFT {
        pre {
            edition >= 1 && edition <= self.maxSupply: "Requested edition is outside of allowed bounds."
            self.mintedEditions[edition] == nil && VouchersSaleManagerHelper.mintedGoatVoucherEditions[edition] == nil : "Requested edition has already been minted"
            GoatedGoatsVouchers.totalSupply + 1 <= self.maxSupply : "Unable to mint any more editions, reached max supply"
        }
        let goatVoucher <- GoatedGoatsVouchers.mint(nftID: edition)
        VouchersSaleManagerHelper.mintedGoatVoucherEditions[edition] = true
        return <-goatVoucher
    }

    // Look for the next available voucher, and mint there
    access(self) fun mintSequentialGoatVoucher(): @NonFungibleToken.NFT {
        var curEditionNumber = VouchersSaleManagerHelper.curSequentialGoatEditionNumber
        while (self.mintedEditions.containsKey(UInt64(curEditionNumber)) ||
                VouchersSaleManagerHelper.mintedGoatVoucherEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        VouchersSaleManagerHelper.setSequentialGoatEditionNumber(curEditionNumber)
        emit PublicGoatVoucherMint(id: UInt64(curEditionNumber))
        return <-self.mintGoatVoucher(edition: UInt64(curEditionNumber))
    }

    // Mint a voucher for goated goats
    access(contract) fun mintPackVoucher(edition: UInt64): @NonFungibleToken.NFT {
        pre {
            edition >= 1 && edition <= self.maxSupply: "Requested edition is outside of allowed bounds."
            self.mintedEditions[edition] == nil && VouchersSaleManagerHelper.mintedPackVoucherEditions[edition] == nil : "Requested edition has already been minted"
            TraitPacksVouchers.totalSupply + 1 <= self.maxSupply : "Unable to mint any more editions, reached max supply"
        }
        let traitPackVoucher <- TraitPacksVouchers.mint(nftID: edition)
        VouchersSaleManagerHelper.mintedPackVoucherEditions[edition] = true
        return <-traitPackVoucher
    }

    // Look for the next available voucher, and mint there
    access(self) fun mintSequentialPackVoucher(): @NonFungibleToken.NFT {
        var curEditionNumber = VouchersSaleManagerHelper.curSequentialPackEditionNumber
        while (self.mintedEditions.containsKey(UInt64(curEditionNumber)) ||
                VouchersSaleManagerHelper.mintedPackVoucherEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        VouchersSaleManagerHelper.setSequentialPackEditionNumber(curEditionNumber)
        emit PublicPackVoucherMint(id: UInt64(curEditionNumber))
        return <-self.mintPackVoucher(edition: UInt64(curEditionNumber))
    }

    // -----------------------------------------------------------------------
    // Public Functions
    // -----------------------------------------------------------------------

    // Accepts payment for vouchers, payment is moved to the `self.paymentReceiver` capability field
    pub fun publicBatchMintSequentialGoatVouchers(buyVault: @FungibleToken.Vault, quantity: UInt64): @NonFungibleToken.Collection {
        pre {
            quantity >= 1 && quantity <= VouchersSaleManagerHelper.goatVoucherMaxQuantityPerMint : "Invalid quantity provided"
            getCurrentBlock().timestamp >= VouchersSaleManagerHelper.goatVoucherSaleStartTime: "Sale has not yet started"
            GoatedGoatsVouchers.totalSupply + quantity <= GoatedGoatsVouchers.maxSupply : "Unable to mint, mint goes above max supply"
        }

        // -- Receive Payments --
        let totalPrice = VouchersSaleManagerHelper.goatVoucherSalePrice * UFix64(quantity)
        // Ensure that the provided balance is equal to our expected price for the NFTs
        assert(totalPrice == buyVault.balance, message: "Invalid amount of Flow provided")
        let flowVault <- buyVault as! @FlowToken.Vault
        self.paymentReceiver.borrow()!.deposit(from: <-flowVault.withdraw(amount: flowVault.balance))
        assert(flowVault.balance == 0.0, message: "Reached unexpected state with payment - balance is not empty")
        destroy flowVault

        // -- Mint the Vouchers --
        // For `quantity` number of NFTs, mint a sequential edition NFT
        let goatedGoatsVoucherCollection <- GoatedGoatsVouchers.createEmptyCollection()
        var i = 0
        while (UInt64(i) < quantity) {
            goatedGoatsVoucherCollection.deposit(token: <-self.mintSequentialGoatVoucher())
            i = i + 1
        }
        assert(goatedGoatsVoucherCollection.getIDs().length == Int(quantity), message: "Failed to mint expected amount of goat vouchers")

        // -- Return the resulting collection --
        return <-goatedGoatsVoucherCollection
    }

    // Accepts payment for vouchers, payment is moved to the `self.paymentReceiver` capability field
    pub fun publicBatchMintSequentialPackVouchers(buyVault: @FungibleToken.Vault, quantity: UInt64): @NonFungibleToken.Collection {
        pre {
            quantity >= 1 && quantity <= VouchersSaleManagerHelper.packVoucherMaxQuantityPerMint : "Invalid quantity provided"
            getCurrentBlock().timestamp >= VouchersSaleManagerHelper.packVoucherSaleStartTime: "Sale has not yet started"
            TraitPacksVouchers.totalSupply + quantity <= TraitPacksVouchers.maxSupply : "Unable to mint, mint goes above max supply"
        }

        // -- Receive Payments --
        let totalPrice = VouchersSaleManagerHelper.packVoucherSalePrice * UFix64(quantity)
        // Ensure that the provided balance is equal to our expected price for the NFTs
        assert(totalPrice == buyVault.balance, message: "Invalid amount of Flow provided")
        let flowVault <- buyVault as! @FlowToken.Vault
        self.paymentReceiver.borrow()!.deposit(from: <-flowVault.withdraw(amount: flowVault.balance))
        assert(flowVault.balance == 0.0, message: "Reached unexpected state with payment - balance is not empty")
        destroy flowVault

        // -- Mint the Vouchers --
        // For `quantity` number of NFTs, mint a sequential edition NFT
        let traitPacksVoucherCollection <- TraitPacksVouchers.createEmptyCollection()
        var i = 0
        while (UInt64(i) < quantity) {
            traitPacksVoucherCollection.deposit(token: <-self.mintSequentialPackVoucher())
            i = i + 1
        }
        assert(traitPacksVoucherCollection.getIDs().length == Int(quantity), message: "Failed to mint expected amount of pack vouchers")

        // -- Return the resulting collection --
        return <-traitPacksVoucherCollection
    }

    // -----------------------------------------------------------------------
    // Deprecated Functions
    // -----------------------------------------------------------------------
    // Mint a voucher for both goated goats and trait packs with the given edition
    //
    // DEPRECATED - use mintGoatVoucher or mintPackVoucher
    access(contract) fun mintVouchers(edition: UInt64): @[NonFungibleToken.NFT] {
        pre {
            edition >= 1 && edition <= self.maxSupply: "Requested edition is outside of allowed bounds."
            self.mintedEditions[edition] == nil : "Requested edition has already been minted"
            self.totalSupply + 1 <= self.maxSupply : "Unable to mint any more editions, reached max supply"
        }
        self.mintedEditions[edition] = true
        self.totalSupply = self.totalSupply + 1
        let goatVoucher <- GoatedGoatsVouchers.mint(nftID: edition)
        let packVoucher <- TraitPacksVouchers.mint(nftID: edition)
        return <-[<-goatVoucher, <-packVoucher]
    }

    // Look for the next available voucher, and mint there
    //
    // DEPRECATED - use mintSequentialGoatVoucher or mintSequentialPackVoucher for specific voucher types
    access(self) fun mintSequentialVouchers(): @[NonFungibleToken.NFT] {
        var curEditionNumber = self.sequentialMintMin
        while (self.mintedEditions.containsKey(UInt64(curEditionNumber))) {
            curEditionNumber = curEditionNumber + 1
        }
        self.sequentialMintMin = curEditionNumber
        emit PublicMint(id: UInt64(curEditionNumber))
        let newVouchers <- self.mintVouchers(edition: UInt64(curEditionNumber))
        return <-newVouchers
    }

    // Accepts payment for vouchers, payment is moved to the `self.paymentReceiver` capability field
    // 
    // DEPRECATED - use publicBatchMintSequential for specific voucher types from functions below
    pub fun publicBatchMintSequentialVouchers(buyVault: @FungibleToken.Vault, quantity: UInt64): @[NonFungibleToken.Collection] {
        pre {
            quantity >= 1 && quantity <= self.maxQuantityPerMint : "Invalid quantity provided"
            getCurrentBlock().timestamp >= self.saleStartTime: "Sale has not yet started"
            self.totalSupply + quantity <= self.maxSupply : "Unable to mint, mint goes above max supply"
        }
        // -- Receive Payments --
        let totalPrice = self.salePrice * UFix64(quantity)
        // Ensure that the provided balance is equal to our expected price for the NFTs
        assert(totalPrice == buyVault.balance, message: "Invalid amount of Flow provided")
        let flowVault <- buyVault as! @FlowToken.Vault
        self.paymentReceiver.borrow()!.deposit(from: <-flowVault.withdraw(amount: flowVault.balance))
        assert(flowVault.balance == 0.0, message: "Reached unexpected state with payment - balance is not empty")
        destroy flowVault

        // -- Mint the Vouchers --
        // For `quantity` number of NFTs, mint a sequential edition NFT
        let goatedGoatsVoucherCollection <- GoatedGoatsVouchers.createEmptyCollection()
        let traitPacksVoucherCollection <- TraitPacksVouchers.createEmptyCollection()
        var i = 0
        while (UInt64(i) < quantity) {
            // nfts[0] is a GoatedGoatsVoucher
            // nfts[1] is a TraitPacksVoucher
            let nfts <- self.mintSequentialVouchers()

            let goatResource <- nfts.remove(at: 0)
            goatedGoatsVoucherCollection.deposit(token: <-goatResource)

            let traitResource <- nfts.remove(at: 0)
            traitPacksVoucherCollection.deposit(token: <-traitResource)

            assert(nfts.length == 0, message: "More NFTs than expected returned from mint.")
            destroy nfts

            i = i + 1
        }
        assert(goatedGoatsVoucherCollection.getIDs().length == Int(quantity), message: "Failed to mint expected amount of goat vouchers")
        assert(traitPacksVoucherCollection.getIDs().length == Int(quantity), message: "Failed to mint expected amount of pack vouchers")

        // -- Return the resulting collections --
        let res: @[NonFungibleToken.Collection] <- []
        res.append(<-goatedGoatsVoucherCollection)
        res.append(<-traitPacksVoucherCollection)
        return <-res
    }

    init() {
        // Non-human modifiable variables
        self.maxSupply = 10000
        self.totalSupply = 0
        self.sequentialMintMin = 1

        // Updateable variables by admin
        self.maxQuantityPerMint = 0
        self.saleStartTime = 4891048813.0
        self.salePrice = 10000000.0

        // Manager resource is only saved to the deploying account's storage
        self.ManagerStoragePath = /storage/GoatedGoatsVouchersManager
        self.account.save(<- create Manager(), to: self.ManagerStoragePath)

        // Start with no existing editions minted
        self.mintedEditions = {}

        // Default payment receiver will be the contract deploying account
        self.paymentReceiver = self.account.getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)

        emit ContractInitialized()
    }
}
