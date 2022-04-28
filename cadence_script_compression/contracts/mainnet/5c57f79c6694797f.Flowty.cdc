import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

import FlowtyUtils from 0x5c57f79c6694797f
import CoatCheck from 0x5c57f79c6694797f

// Flowty
//
// A smart contract responsible for the main lending flows. 
// It is facilitating the lending deal, allowing borrowers and lenders 
// to be sure that the deal would be executed on their agreed terms.
// 
// Each account that wants to list a loan installs a Storefront,
// and lists individual loan within that Storefront as Listings.
// There is one Storefront per account, it handles loans of all NFT types
// for that account.
//
// Each Listing can have one or more "cut"s of the requested loan amount that
// goes to one or more addresses. Cuts are used to pay listing fees
// or other considerations.
// Each NFT may be listed in one or more Listings, the validity of each
// Listing can easily be checked.
// 
// Lenders can watch for Listing events and check the NFT type and
// ID to see if they wish to fund the listed loan.
//
pub contract Flowty {
    // FlowtyInitialized
    // This contract has been deployed.
    // Event consumers can now expect events from this contract.
    //
    pub event FlowtyInitialized()

    // FlowtyStorefrontInitialized
    // A FlowtyStorefront resource has been created.
    // Event consumers can now expect events from this FlowtyStorefront.
    // Note that we do not specify an address: we cannot and should not.
    // Created resources do not have an owner address, and may be moved
    // after creation in ways we cannot check.
    // ListingAvailable events can be used to determine the address
    // of the owner of the FlowtyStorefront (...its location) at the time of
    // the listing but only at that precise moment in that precise transaction.
    // If the seller moves the FlowtyStorefront while the listing is valid, 
    // that is on them.
    //
    pub event FlowtyStorefrontInitialized(flowtyStorefrontResourceID: UInt64)

    // FlowtyMarketplaceInitialized
    // A FlowtyMarketplace resource has been created.
    // Event consumers can now expect events from this FlowtyStorefront.
    // Note that we do not specify an address: we cannot and should not.
    // Created resources do not have an owner address, and may be moved
    // after creation in ways we cannot check.
    // ListingAvailable events can be used to determine the address
    // of the owner of the FlowtyStorefront (...its location) at the time of
    // the listing but only at that precise moment in that precise transaction.
    // If the seller moves the FlowtyStorefront while the listing is valid, 
    // that is on them.
    //
    pub event FlowtyMarketplaceInitialized(flowtyMarketplaceResourceID: UInt64)

    // FlowtyStorefrontDestroyed
    // A FlowtyStorefront has been destroyed.
    // Event consumers can now stop processing events from this FlowtyStorefront.
    // Note that we do not specify an address.
    //
    pub event FlowtyStorefrontDestroyed(flowtyStorefrontResourceID: UInt64)

    // FlowtyMarketplaceDestroyed
    // A FlowtyMarketplace has been destroyed.
    // Event consumers can now stop processing events from this FlowtyMarketplace.
    // Note that we do not specify an address.
    //
    pub event FlowtyMarketplaceDestroyed(flowtyStorefrontResourceID: UInt64)

    // ListingAvailable
    // A listing has been created and added to a FlowtyStorefront resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // FlowtyMarketplace workflow, so be careful to check when using them.
    //
    pub event ListingAvailable(
        flowtyStorefrontAddress: Address,
        flowtyStorefrontID: UInt64, 
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        amount: UFix64,
        interestRate: UFix64,
        term: UFix64,
        enabledAutoRepayment: Bool,
        royaltyRate: UFix64,
        expiresAfter: UFix64,
        paymentTokenType: Type
    )

    // ListingCompleted
    // The listing has been resolved. It has either been funded, or removed and destroyed.
    //
    pub event ListingCompleted(
        listingResourceID: UInt64, 
        flowtyStorefrontID: UInt64, 
        funded: Bool)

    // FundingAvailable
    // A funding has been created and added to a FlowtyStorefront resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // FlowtyMarketplace workflow, so be careful to check when using them.
    //
    pub event FundingAvailable(
        fundingResourceID: UInt64,
        listingResourceID: UInt64,
        borrower: Address,
        lender: Address,
        nftID: UInt64,
        repaymentAmount: UFix64,
        enabledAutoRepayment: Bool
    )

    // FundingRepaid
    // A funding has been repaid.
    //
    pub event FundingRepaid(
        fundingResourceID: UInt64,
        listingResourceID: UInt64,
        borrower: Address,
        lender: Address,
        nftID: UInt64,
        repaymentAmount: UFix64
    )

    // FundingSettled
    // A funding has been settled.
    //
    pub event FundingSettled(
        fundingResourceID: UInt64,
        listingResourceID: UInt64,
        borrower: Address,
        lender: Address,
        nftID: UInt64,
        repaymentAmount: UFix64
    )

    pub event CollectionSupportChanged(
        collectionIdentifier: String,
        state: Bool
    )

    pub event RoyaltyAdded(
        collectionIdentifier: String,
        rate: UFix64
    )

    pub event RoyaltyEscrow(
        fundingResourceID: UInt64,
        listingResourceID: UInt64,
        lender: Address,
        amount: UFix64
    )

    // FlowtyStorefrontStoragePath
    // The location in storage that a FlowtyStorefront resource should be located.
    pub let FlowtyStorefrontStoragePath: StoragePath

    // FlowtyMarketplaceStoragePath
    // The location in storage that a FlowtyMarketplace resource should be located.
    pub let FlowtyMarketplaceStoragePath: StoragePath

    // FlowtyStorefrontPublicPath
    // The public location for a FlowtyStorefront link.
    pub let FlowtyStorefrontPublicPath: PublicPath

    // FlowtyMarketplacePublicPath
    // The public location for a FlowtyMarketplace link.
    pub let FlowtyMarketplacePublicPath: PublicPath

    // FlowtyAdminStoragePath
    // The location in storage that an FlowtyAdmin resource should be located.
    pub let FlowtyAdminStoragePath: StoragePath

    // FusdVaultStoragePath
    // The location in storage that an FUSD Vault resource should be located.
    pub let FusdVaultStoragePath: StoragePath

    // FusdReceiverPublicPath
    // The public location for a FUSD Receiver link.
    pub let FusdReceiverPublicPath: PublicPath

    // FusdBalancePublicPath
    // The public location for a FUSD Balance link.
    pub let FusdBalancePublicPath: PublicPath

    // ListingFee
    // The fixed fee in FUSD for a listing.
    pub var ListingFee: UFix64

    // FundingFee
    // The percentage fee on funding, a number between 0 and 1.
    pub var FundingFee: UFix64

    // SuspendedFundingPeriod
    // The suspended funding period in seconds(started on listing). 
    // So that the borrower has some time to delist it.
    pub var SuspendedFundingPeriod: UFix64

    // A dictionary for the Collection to royalty configuration.
    access(account) var Royalties: {String:Royalty}
    access(account) var TokenPaths: {String:PublicPath}

    // The collections which are allowed to be used as collateral
    access(account) var SupportedCollections: {String:Bool}

    // PaymentCut
    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a tx is executed.
    //
    pub struct PaymentCut {
        // The receiver for the payment.
        // Note that we do not store an address to find the Vault that this represents,
        // as the link or resource that we fetch in this way may be manipulated,
        // so to find the address that a cut goes to you must get this struct and then
        // call receiver.borrow()!.owner.address on it.
        // This can be done efficiently in a script.
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        // The amount of the payment FungibleToken that will be paid to the receiver.
        pub let amount: UFix64

        // initializer
        //
        init(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
            self.receiver = receiver
            self.amount = amount
        }
    }

    pub struct Royalty {
        // The percentage points that should go to the collection owner
        // In the event of a loan default
        pub let Rate: UFix64
        pub let Address: Address 
        pub let ReceiverPaths: {String: PublicPath}

        init(rate: UFix64, address: Address) {
            self.Rate = rate
            self.Address = address
            self.ReceiverPaths = {}
        }

        access(account) fun addVault(path: PublicPath, vaultType: Type) {
            self.ReceiverPaths[vaultType.identifier] = path
        }
    }

    // ListingDetails
    // A struct containing a Listing's data.
    //
    pub struct ListingDetails {
        // The FlowtyStorefront that the Listing is stored in.
        // Note that this resource cannot be moved to a different FlowtyStorefront
        pub var flowtyStorefrontID: UInt64
        // Whether this listing has been funded or not.
        pub var funded: Bool
        // The Type of the NonFungibleToken.NFT that is being listed.
        pub let nftType: Type
        // The ID of the NFT within that type.
        pub let nftID: UInt64
        // The amount of the requested loan.
        pub let amount: UFix64
        // The interest rate in %, a number between 0 and 1.
        pub let interestRate: UFix64
        //The term in seconds for this listing.
        pub var term: UFix64
        // The Type of the FungibleToken that fundings must be made in.
        pub let paymentVaultType: Type
        // This specifies the division of payment between recipients.
        access(self) let paymentCuts: [PaymentCut]
        //The time the funding start at
        pub var listedTime: UFix64
        // The royalty rate needed as a deposit for this loan to be funded
        pub var royaltyRate: UFix64
        // The number of seconds this listing is valid for
        pub var expiresAfter: UFix64

        // getPaymentCuts
        // Returns payment cuts
        pub fun getPaymentCuts(): [PaymentCut] {
            return self.paymentCuts
        }

        pub fun getTotalPayment(): UFix64 {
            return self.amount * (1.0 + (self.interestRate * Flowty.FundingFee) + self.royaltyRate)
        }

        // setToFunded
        // Irreversibly set this listing as funded.
        //
        access(contract) fun setToFunded() {
            self.funded = true
        }

        // initializer
        //
        init (
            nftType: Type,
            nftID: UInt64,
            amount: UFix64,
            interestRate: UFix64,
            term: UFix64,
            paymentVaultType: Type,
            paymentCuts: [PaymentCut],
            flowtyStorefrontID: UInt64,
            expiresAfter: UFix64
        ) {
            self.flowtyStorefrontID = flowtyStorefrontID
            self.funded = false
            self.nftType = nftType
            self.nftID = nftID
            self.amount = amount
            self.interestRate = interestRate
            self.term = term
            self.paymentVaultType = paymentVaultType
            self.listedTime = getCurrentBlock().timestamp
            self.royaltyRate = Flowty.Royalties[self.nftType.identifier]!.Rate
            self.expiresAfter = expiresAfter

            assert(paymentCuts.length > 0, message: "Listing must have at least one payment cut recipient")
            self.paymentCuts = paymentCuts

            // Calculate the total price from the cuts
            var cutsAmount = 0.0
            // Perform initial check on capabilities, and calculate payment price from cut amounts.
            for cut in self.paymentCuts {
                // Make sure we can borrow the receiver.
                cut.receiver.borrow()
                    ?? panic("Cannot borrow receiver")
                // Add the cut amount to the total price
                cutsAmount = cutsAmount + cut.amount
            }

            assert(cutsAmount > 0.0, message: "Listing must have non-zero requested amount")
        }
    }

    // ListingPublic
    // An interface providing a useful public interface to a Listing.
    //
    pub resource interface ListingPublic {
        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT

        // fund
        // Fund the listing.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun fund(payment: @FungibleToken.Vault, 
            lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>, 
            lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>)

        // getDetails
        //
        pub fun getDetails(): ListingDetails

        // suspensionTimeRemaining
        // 
        pub fun suspensionTimeRemaining() : Fix64

        // remainingTimeToFund
        //
        pub fun remainingTimeToFund(): Fix64

        // isFundingEnabled
        //
        pub fun isFundingEnabled(): Bool 
    }


    // Listing
    // A resource that allows an NFT to be fund for an amount of a given FungibleToken,
    // and for the proceeds of that payment to be split between several recipients.
    // 
    pub resource Listing: ListingPublic {
        // The simple (non-Capability, non-complex) details of the listing
        access(self) let details: ListingDetails

        // A capability allowing this resource to withdraw the NFT with the given ID from its collection.
        // This capability allows the resource to withdraw *any* NFT, so you should be careful when giving
        // such a capability to a resource and always check its code to make sure it will use it in the
        // way that it claims.
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>

        // A capability allowing this resource to access the owner's NFT public collection 
        access(contract) let nftPublicCollectionCapability: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>


        // A capability allowing this resource to withdraw `FungibleToken`s from borrower account.
        // This capability allows loan repayment if there is system downtime, which will prevent NFT losing.
        // NOTE: This variable cannot be renamed but it can allow any FungibleToken.
        access(contract) let fusdProviderCapability: Capability<&{FungibleToken.Provider}>?

        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT {
            let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: self.getDetails().nftID)
            //- CANNOT DO THIS IN PRECONDITION: "member of restricted type is not accessible: isInstance"
            //  result.isInstance(self.getDetails().nftType): "token has wrong type"
            assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
            assert(ref.id == self.getDetails().nftID, message: "token has wrong ID")
            return ref
        }

        // getDetails
        // Get the details of the current state of the Listing as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources).
        //
        pub fun getDetails(): ListingDetails {
            return self.details
        }

        // fund
        // Fund the listing.
        // This pays the beneficiaries and move the NFT to the funding resource stored in the marketplace account.
        //
        pub fun fund(payment: @FungibleToken.Vault, 
            lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>,
            lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>) {
            pre {
                self.isFundingEnabled(): "Funding is not enabled or this listing has expired"
                self.details.funded == false: "listing has already been funded"
                payment.isInstance(self.details.paymentVaultType): "payment vault is not requested fungible token"
                Flowty.Royalties[self.details.nftType.identifier] != nil: "royalty information not found for given collection"
                payment.balance == self.details.getTotalPayment(): "payment vault does not contain requested amount"
            }

            // Make sure the listing cannot be funded again.
            self.details.setToFunded()

            // Fetch the token to return to the purchaser.
            let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)
            // Neither receivers nor providers are trustworthy, they must implement the correct
            // interface but beyond complying with its pre/post conditions they are not gauranteed
            // to implement the functionality behind the interface in any given way.
            // Therefore we cannot trust the Collection resource behind the interface,
            // and we must check the NFT resource it gives us to make sure that it is the correct one.
            assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")

            // Rather than aborting the transaction if any receiver is absent when we try to pay it,
            // we send the cut to the first valid receiver.
            // The first receiver should therefore either be the borrower, or an agreed recipient for
            // any unpaid cuts.
            var residualReceiver: &{FungibleToken.Receiver}? = nil

            // Pay each beneficiary their amount of the payment.
            for cut in self.details.getPaymentCuts() {
                if let receiver = cut.receiver.borrow() {
                   let paymentCut <- payment.withdraw(amount: cut.amount)
                    receiver.deposit(from: <-paymentCut)
                    if (residualReceiver == nil) {
                        residualReceiver = receiver
                    }
                }
            }

            // Funding fee
            let fundingFeeAmount = self.details.amount * self.details.interestRate * Flowty.FundingFee
            let fundingFee <- payment.withdraw(amount: fundingFeeAmount)
            let feeTokenPath = Flowty.TokenPaths[self.details.paymentVaultType.identifier]!
            let flowtyFeeReceiver = Flowty.account.getCapability<&AnyResource{FungibleToken.Receiver}>(feeTokenPath)!.borrow()
                ?? panic("Missing or mis-typed FungibleToken Reveiver")
            flowtyFeeReceiver.deposit(from: <-fundingFee)

            // Royalty
            // Deposit royalty amount 
            let royalty = self.details.royaltyRate
            let royaltyVault <- payment.withdraw(amount: self.details.amount * royalty)

            assert(residualReceiver != nil, message: "No valid payment receivers")

            // At this point, if all recievers were active and availabile, then the payment Vault will have
            // zero tokens left, and this will functionally be a no-op that consumes the empty vault
            residualReceiver!.deposit(from: <-payment)

            let listingResourceID = self.uuid

            // If the listing is funded, we regard it as completed here.
            // Otherwise we regard it as completed in the destructor.
            emit ListingCompleted(
                listingResourceID: listingResourceID,
                flowtyStorefrontID: self.details.flowtyStorefrontID,
                funded: self.details.funded
            )

            let repaymentAmount = self.details.amount + self.details.amount * self.details.interestRate

            let marketplace = Flowty.borrowMarketplace()
            marketplace.createFunding(
                flowtyStorefrontID: self.details.flowtyStorefrontID, 
                listingResourceID: listingResourceID, 
                ownerNFTCollection: self.nftPublicCollectionCapability, 
                lenderNFTCollection: lenderNFTCollection, 
                NFT: <-nft, 
                paymentVaultType: self.details.paymentVaultType,
                lenderFungibleTokenReceiver: lenderFungibleTokenReceiver,
                repaymentAmount: repaymentAmount,
                term: self.details.term,
                fusdProviderCapability: self.fusdProviderCapability,
                royaltyVault: <-royaltyVault,
                listingDetails: self.getDetails()
            )
        }

        // suspensionTimeRemaining
        // The remaining time. This can be negative if is expired
        pub fun suspensionTimeRemaining() : Fix64 {
            let listedTime = self.details.listedTime
            let currentTime = getCurrentBlock().timestamp

            let remaining = Fix64(listedTime+Flowty.SuspendedFundingPeriod) - Fix64(currentTime)

            return remaining
        }

        // remainingTimeToFund
        // The time in seconds left until this listing is no longer valid
        pub fun remainingTimeToFund(): Fix64 {
            let listedTime = self.details.listedTime
            let currentTime = getCurrentBlock().timestamp
            let remaining = Fix64(listedTime + self.details.expiresAfter) - Fix64(currentTime)
            return remaining
        }

        // isFundingEnabled
        pub fun isFundingEnabled(): Bool {
            let timeRemaining = self.suspensionTimeRemaining()
            let listingTimeRemaining = self.remainingTimeToFund()
            return timeRemaining < Fix64(0.0) && listingTimeRemaining > Fix64(0.0)
        }

        // initializer
        //
        init (
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftPublicCollectionCapability: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            nftType: Type,
            nftID: UInt64,
            amount: UFix64,
            interestRate: UFix64,
            term: UFix64,
            paymentVaultType: Type,
            paymentCuts: [PaymentCut],
            flowtyStorefrontID: UInt64,
            expiresAfter: UFix64
        ) {
            // Store the sale information
            self.details = ListingDetails(
                nftType: nftType,
                nftID: nftID,
                amount: amount,
                interestRate: interestRate,
                term: term,
                paymentVaultType: paymentVaultType,
                paymentCuts: paymentCuts,
                flowtyStorefrontID: flowtyStorefrontID,
                expiresAfter: expiresAfter
            )

            // Store the NFT provider
            self.nftProviderCapability = nftProviderCapability

            self.fusdProviderCapability = fusdProviderCapability

            self.nftPublicCollectionCapability = nftPublicCollectionCapability

            // Check that the provider contains the NFT.
            // We will check it again when the token is funded.
            // We cannot move this into a function because initializers cannot call member functions.
            let provider = self.nftProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow nftProviderCapability")

            // This will precondition assert if the token is not available.
            let nft = provider!.borrowNFT(id: self.details.nftID)
            assert(nft.isInstance(self.details.nftType), message: "token is not of specified type")
            assert(nft.id == self.details.nftID, message: "token does not have specified ID")
        }

        // destructor
        //
        destroy () {
            // We regard the listing as completed here.
            emit ListingCompleted(
                listingResourceID: self.uuid,
                flowtyStorefrontID: self.details.flowtyStorefrontID,
                funded: self.details.funded
            )
        }
    }

    // FundingDetails
    // A struct containing a Fundings's data.
    //
    pub struct FundingDetails {
        // The FlowtyStorefront that the Funding is stored in.
        // Note that this resource cannot be moved to a different FlowtyStorefront
        pub var flowtyStorefrontID: UInt64
        pub var listingResourceID: UInt64

        // Whether this funding has been repaid or not.
        pub var repaid: Bool

        // Whether this funding has been settled or not.
        pub var settled: Bool

        // The Type of the FungibleToken that fundings must be repaid.
        pub let paymentVaultType: Type

        // The amount that must be repaid in the specified FungibleToken.
        pub let repaymentAmount: UFix64

        // the time the funding start at
        pub var startTime: UFix64

        // The length in seconds for this funding
        pub var term: UFix64

        // setToRepaid
        // Irreversibly set this funding as repaid.
        //
        access(contract) fun setToRepaid() {
            self.repaid = true
        }

        // setToSettled
        // Irreversibly set this funding as settled.
        //
        access(contract) fun setToSettled() {
            self.settled = true
        }

        // initializer
        //
        init (
            flowtyStorefrontID: UInt64,
            listingResourceID: UInt64,
            paymentVaultType: Type,
            repaymentAmount: UFix64, 
            term: UFix64
        ) {
            self.flowtyStorefrontID = flowtyStorefrontID
            self.listingResourceID = listingResourceID
            self.paymentVaultType = paymentVaultType
            self.repaid = false
            self.settled = false
            self.repaymentAmount = repaymentAmount
            self.term = term
            self.startTime = getCurrentBlock().timestamp
        }
    }

    // FundingPublic
    // An interface providing a useful public interface to a Funding.
    //
    pub resource interface FundingPublic {

        // repay
        //
        pub fun repay(payment: @FungibleToken.Vault)

        // getDetails
        //
        pub fun getDetails(): FundingDetails

        // get the listing details for this loan
        //
        pub fun getListingDetails(): Flowty.ListingDetails

        // timeRemaining
        // 
        pub fun timeRemaining() : Fix64

        // isFundingExpired
        //
        pub fun isFundingExpired(): Bool 

        // get the amount stored in a vault for royalty payouts
        //
        pub fun getRoyaltyAmount(): UFix64?
    }

    // Funding
    // 
    pub resource Funding: FundingPublic {
        // The simple (non-Capability, non-complex) details of the listing
        access(self) let details: FundingDetails
        access(self) let listingDetails: ListingDetails

        // A capability allowing this resource to access the owner's NFT public collection 
        access(contract) let ownerNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>

        // A capability allowing this resource to access the lender's NFT public collection 
        access(contract) let lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>

        // The receiver for the repayment.
        access(contract) let lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>

        // NFT escrow
        access(contract) var NFT: @NonFungibleToken.NFT?

        // FUSD Allowance
        access(contract) let fusdProviderCapability: Capability<&{FungibleToken.Provider}>?

        // royalty payment vault to be deposited to the specified desination on repayment or default
        access(contract) var royaltyVault: @FungibleToken.Vault?

        // getDetails
        // Get the details of the current state of the Listing as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources).
        //
        pub fun getDetails(): FundingDetails {
            return self.details
        }

        pub fun getListingDetails(): ListingDetails {
            return self.listingDetails
        }

        pub fun getRoyaltyAmount(): UFix64? {
            return self.royaltyVault?.balance
        }

        // repay
        // Repay the funding.
        // This pays the lender and returns the NFT to the owner.
        //
        pub fun repay(payment: @FungibleToken.Vault) {
            pre {
                !self.isFundingExpired(): "the loan has expired"
                self.details.repaid == false: "funding has already been repaid"
                payment.isInstance(self.details.paymentVaultType): "payment vault is not requested fungible token"
                payment.balance == self.details.repaymentAmount: "payment vault does not contain requested price"
            }

            self.details.setToRepaid()

            let NFT <- self.NFT <- nil
            let nftID = NFT?.id

            FlowtyUtils.trySendNFT(nft: <-NFT!, receiver: self.ownerNFTCollection)

            let royaltyVault <- self.royaltyVault <- nil
            let repaymentVault <- payment
            repaymentVault.deposit(from: <-royaltyVault!)

            FlowtyUtils.trySendFungibleTokenVault(vault: <-repaymentVault, receiver: self.lenderFungibleTokenReceiver)

            let borrower = self.ownerNFTCollection.address
            let lender = self.lenderFungibleTokenReceiver.address
            emit FundingRepaid(
                fundingResourceID: self.uuid, 
                listingResourceID: self.details.listingResourceID, 
                borrower: borrower,
                lender: lender,
                nftID: nftID!,
                repaymentAmount: self.details.repaymentAmount
            )
        }

        // repay
        // Repay the funding with borrower permit.
        // This pays the lender and returns the NFT to the owner using FUSD allowance from borrower account.
        //
        pub fun repayWithPermit() {
            pre {
                self.details.repaid == false: "funding has already been repaid"
                self.details.settled == false: "funding has already been settled"
                self.fusdProviderCapability != nil: "listing is created without FUSD allowance"
            }

            self.details.setToRepaid()

            let NFT <- self.NFT <- nil
            let nftID = NFT?.id

            let borrowerVault = self.fusdProviderCapability!.borrow()!
            let payment <- borrowerVault.withdraw(amount: self.details.repaymentAmount)

            FlowtyUtils.trySendNFT(nft: <-NFT!, receiver: self.ownerNFTCollection)

            let royaltyVault <- self.royaltyVault <- nil
            let repaymentVault <- payment
            repaymentVault.deposit(from: <-royaltyVault!)

            FlowtyUtils.trySendFungibleTokenVault(vault: <-repaymentVault, receiver: self.lenderFungibleTokenReceiver)
            let borrower = self.ownerNFTCollection.address
            let lender = self.lenderFungibleTokenReceiver.address

            emit FundingRepaid(
                fundingResourceID: self.uuid, 
                listingResourceID: self.details.listingResourceID, 
                borrower: borrower,
                lender: lender,
                nftID: nftID!,
                repaymentAmount: self.details.repaymentAmount
            )
        }

        // settleFunding
        // Settle the different statuses responsible for the repayment and claiming processes.
        // NFT is moved to the lender, because the borrower hasn't repaid the loan.
        //
        pub fun settleFunding() {
            pre {
                self.isFundingExpired(): "the loan hasn't expired"
                self.details.repaid == false: "funding has already been repaid"
                self.details.settled == false: "funding has already been settled"
            }

            self.details.setToSettled()

            // Move NFT to the lender account
            let NFT <- self.NFT <- nil
            let nftID = NFT?.id
            assert(nftID != nil, message: "NFT is already moved")

            let royalty = Flowty.getRoyalty(nftTypeIdentifier: self.getListingDetails().nftType.identifier)
            let royaltyTokenPath = Flowty.TokenPaths[self.details.paymentVaultType.identifier]!
            let receiverCap = getAccount(royalty.Address).getCapability<&AnyResource{FungibleToken.Receiver}>(royaltyTokenPath)

            let royaltyVault <- self.royaltyVault <- nil

            FlowtyUtils.trySendFungibleTokenVault(vault: <-royaltyVault!, receiver: receiverCap)
            FlowtyUtils.trySendNFT(nft: <-NFT!, receiver: self.lenderNFTCollection)

            let lender = self.lenderNFTCollection.address
            let borrower = self.ownerNFTCollection.address

            emit FundingSettled(
                fundingResourceID: self.uuid, 
                listingResourceID: self.details.listingResourceID, 
                borrower: borrower,
                lender: lender,
                nftID: nftID!,
                repaymentAmount: self.details.repaymentAmount
            )
        }

        // timeRemaining
        // The remaining time. This can be negative if is expired
        pub fun timeRemaining() : Fix64 {
            let fundingTerm = self.details.term

            let startTime = self.details.startTime
            let currentTime = getCurrentBlock().timestamp

            let remaining = Fix64(startTime+fundingTerm) - Fix64(currentTime)

            return remaining
        }

        // isFundingExpired
        pub fun isFundingExpired(): Bool {
            let timeRemaining= self.timeRemaining()
            return timeRemaining < Fix64(0.0)
        }

        // initializer
        //
        init (
            flowtyStorefrontID: UInt64,
            listingResourceID: UInt64,
            ownerNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            NFT: @NonFungibleToken.NFT,
            paymentVaultType: Type,
            repaymentAmount: UFix64,
            lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>,
            term: UFix64,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            royaltyVault: @FungibleToken.Vault?,
            listingDetails: ListingDetails
        ) {
            self.ownerNFTCollection = ownerNFTCollection
            self.lenderNFTCollection = lenderNFTCollection
            self.lenderFungibleTokenReceiver = lenderFungibleTokenReceiver
            self.fusdProviderCapability = fusdProviderCapability
            self.listingDetails = listingDetails
            self.NFT <-NFT
            self.royaltyVault <-royaltyVault 

            // Store the detailed information
            self.details = FundingDetails(
                flowtyStorefrontID: flowtyStorefrontID,
                listingResourceID: listingResourceID,
                paymentVaultType: paymentVaultType,
                repaymentAmount: repaymentAmount,
                term: term
            )
        }

        destroy() {
            // send the NFT back to the owner
            if self.NFT != nil {
                let NFT <- self.NFT <- nil
                self.ownerNFTCollection.borrow()!.deposit(token: <-NFT!)
            }
            destroy self.NFT

            if self.royaltyVault != nil {
                let royaltyVault <- self.royaltyVault <- nil
                self.lenderFungibleTokenReceiver.borrow()!.deposit(from: <-royaltyVault!)
            }
            destroy self.royaltyVault
        }
    }

    // FlowtyMarketplaceManager
    // An interface for adding and removing Fundings within a FlowtyMarketplace,
    // intended for use by the FlowtyStorefront's own
    //
    pub resource interface FlowtyMarketplaceManager {
        // createFunding
        // Allows the FlowtyMarketplace owner to create and insert Fundings.
        //
        access(contract) fun createFunding(
            flowtyStorefrontID: UInt64, 
            listingResourceID: UInt64,
            ownerNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            NFT: @NonFungibleToken.NFT,
            paymentVaultType: Type,
            lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>,
            repaymentAmount: UFix64,
            term: UFix64,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            royaltyVault: @FungibleToken.Vault?,
            listingDetails: ListingDetails
        ): UInt64
        // removeFunding
        // Allows the FlowtyMarketplace owner to remove any funding.
        //
        pub fun removeFunding(fundingResourceID: UInt64)

        pub fun borrowPrivateFunding(fundingResourceID: UInt64): &Funding?
    }

    // FlowtyMarketplacePublic
    // An interface to allow listing and borrowing Listings, and funding loans via Listings
    // in a FlowtyStorefront.
    //
    pub resource interface FlowtyMarketplacePublic {
        pub fun getFundingIDs(): [UInt64]
        pub fun borrowFunding(fundingResourceID: UInt64): &Funding{FundingPublic}?
        pub fun getAllowedCollections(): [String]
    }

    // FlowtyStorefront
    // A resource that allows its owner to manage a list of Listings, and anyone to interact with them
    // in order to query their details and fund the loans that they represent.
    //
    pub resource FlowtyMarketplace : FlowtyMarketplaceManager, FlowtyMarketplacePublic {
        // The dictionary of Fundings uuids to Funding resources.
        access(self) var fundings: @{UInt64: Funding}

        // insert
        // Create and publish a funding for an NFT.
        //
        access(contract) fun createFunding(
            flowtyStorefrontID: UInt64, 
            listingResourceID: UInt64,
            ownerNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            lenderNFTCollection: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            NFT: @NonFungibleToken.NFT,
            paymentVaultType: Type,
            lenderFungibleTokenReceiver: Capability<&{FungibleToken.Receiver}>,
            repaymentAmount: UFix64,
            term: UFix64,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            royaltyVault: @FungibleToken.Vault?,
            listingDetails: ListingDetails
         ): UInt64 {
            // FundingAvailable event fields
            let nftID = NFT.id

            let lenderVaultCap = lenderFungibleTokenReceiver.borrow()!
            let lender = lenderVaultCap.owner!.address

            let borrowerNFTCollectionCap = ownerNFTCollection.borrow()!
            let borrower = borrowerNFTCollectionCap.owner!.address

            // Create funding resource
            let funding <- create Funding(
                flowtyStorefrontID: flowtyStorefrontID,
                listingResourceID: listingResourceID,
                ownerNFTCollection: ownerNFTCollection,
                lenderNFTCollection: lenderNFTCollection,
                NFT: <-NFT,
                paymentVaultType: paymentVaultType,
                repaymentAmount: repaymentAmount,
                lenderFungibleTokenReceiver: lenderFungibleTokenReceiver,
                term: term,
                fusdProviderCapability: fusdProviderCapability,
                royaltyVault: <-royaltyVault,
                listingDetails: listingDetails
            )

            let fundingResourceID = funding.uuid

            // Add the new Funding to the dictionary.
            let oldFunding <- self.fundings[fundingResourceID] <- funding
            // Note that oldFunding will always be nil, but we have to handle it.
            destroy oldFunding

            let enabledAutoRepayment = fusdProviderCapability != nil

            emit FundingAvailable(
                fundingResourceID: fundingResourceID, 
                listingResourceID: listingResourceID, 
                borrower: borrower,
                lender: lender,
                nftID: nftID,
                repaymentAmount: repaymentAmount,
                enabledAutoRepayment: enabledAutoRepayment
            )

            return fundingResourceID
        }

        // removeFunding
        // Remove a Funding.
        //
        pub fun removeFunding(fundingResourceID: UInt64) {
            let funding <- self.fundings.remove(key: fundingResourceID)
                ?? panic("missing Funding")
    
            assert(funding.getDetails().repaid == true || funding.getDetails().settled == true, message: "funding is not repaid or settled")

            // This will emit a FundingCompleted event.
            destroy funding
        }

        // getFundingIDs
        // Returns an array of the Funding resource IDs that are in the collection
        //
        pub fun getFundingIDs(): [UInt64] {
            return self.fundings.keys
        }

        // borrowFunding
        // Returns a read-only view of the Funding for the given fundingID if it is contained by this collection.
        //
        pub fun borrowFunding(fundingResourceID: UInt64): &Funding{FundingPublic}? {
            if self.fundings[fundingResourceID] != nil {
                return &self.fundings[fundingResourceID] as! &Funding{FundingPublic}
            } else {
                return nil
            }
        }

        // borrowPrivateFunding
        // Returns a private view of the Funding for the given fundingID if it is contained by this collection.
        //
        pub fun borrowPrivateFunding(fundingResourceID: UInt64): &Funding? {
            if self.fundings[fundingResourceID] != nil {
                return &self.fundings[fundingResourceID] as! &Funding
            } else {
                return nil
            }
        }

        pub fun getAllowedCollections(): [String] {
            return Flowty.SupportedCollections.keys
        }

        // destructor
        //
        destroy () {
            destroy self.fundings

            // Let event consumers know that this marketplace will no longer exist
            emit FlowtyMarketplaceDestroyed(flowtyStorefrontResourceID: self.uuid)
        }

        // constructor
        //
        init () {
            self.fundings <- {}

            // Let event consumers know that this storefront exists
            emit FlowtyMarketplaceInitialized(flowtyMarketplaceResourceID: self.uuid)
        }
    }

    // FlowtyStorefrontManager
    // An interface for adding and removing Listings within a FlowtyStorefront,
    // intended for use by the FlowtyStorefront's own
    pub resource interface FlowtyStorefrontManager {
        // createListing
        // Allows the FlowtyStorefront owner to create and insert Listings.
        //
        pub fun createListing(
            payment: @FungibleToken.Vault,
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftPublicCollectionCapability: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            nftType: Type,
            nftID: UInt64,
            amount: UFix64,
            interestRate: UFix64,
            term: UFix64,
            paymentVaultType: Type,
            paymentCuts: [PaymentCut],
            expiresAfter: UFix64
        ): UInt64
        // removeListing
        // Allows the FlowtyStorefront owner to remove any sale listing, accepted or not.
        //
        pub fun removeListing(listingResourceID: UInt64)
    }

    // FlowtyStorefrontPublic
    // An interface to allow listing and borrowing Listings, and funding loans via Listings
    // in a FlowtyStorefront.
    //
    pub resource interface FlowtyStorefrontPublic {
        pub fun getListingIDs(): [UInt64]
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}?
        pub fun cleanup(listingResourceID: UInt64)
        pub fun getRoyalties(): {String:Flowty.Royalty}
   }

    // FlowtyStorefront
    // A resource that allows its owner to manage a list of Listings, and anyone to interact with them
    // in order to query their details and fund the loans that they represent.
    //
    pub resource FlowtyStorefront : FlowtyStorefrontManager, FlowtyStorefrontPublic {
        // The dictionary of Listing uuids to Listing resources.
        access(self) var listings: @{UInt64: Listing}

        // insert
        // Create and publish a Listing for an NFT.
        //
         pub fun createListing(
            payment: @FungibleToken.Vault,
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftPublicCollectionCapability: Capability<&AnyResource{NonFungibleToken.CollectionPublic}>,
            fusdProviderCapability: Capability<&{FungibleToken.Provider}>?,
            nftType: Type,
            nftID: UInt64,
            amount: UFix64,
            interestRate: UFix64,
            term: UFix64,
            paymentVaultType: Type,
            paymentCuts: [PaymentCut],
            expiresAfter: UFix64
         ): UInt64 {
             pre {
                // We don't allow all tokens to be used as payment. Check that the provided one is supported.
                FlowtyUtils.isTokenSupported(type: paymentVaultType): "provided payment type is not supported"
                // make sure that the FUSD vault has at least the listing fee
                payment.balance == Flowty.ListingFee: "payment vault does not contain requested listing fee amount"
                // ensure that this nft type is supported
                Flowty.SupportedCollections[nftType.identifier] == nil: "nftType is not supported"
                // check that the repayment token type is the same as the payment token if repayment is not nil
                fusdProviderCapability == nil || fusdProviderCapability!.borrow()!.isInstance(paymentVaultType): "repayment vault type and payment vault type do not match"
                // There are no listing fees right now so this will ensure that no one attempts to send any
                payment.balance == 0.0: "no listing fee required"
                // make sure the payment type is the same as paymentVaultType
                payment.getType() == paymentVaultType: "payment type and paymentVaultType do not match"
            }

            let listing <- create Listing(
                nftProviderCapability: nftProviderCapability,
                nftPublicCollectionCapability: nftPublicCollectionCapability,
                fusdProviderCapability: fusdProviderCapability,
                nftType: nftType,
                nftID: nftID,
                amount: amount,
                interestRate: interestRate,
                term: term,
                paymentVaultType: paymentVaultType,
                paymentCuts: paymentCuts,
                flowtyStorefrontID: self.uuid,
                expiresAfter: expiresAfter
            )

            let listingResourceID = listing.uuid
            let royaltyRate = listing.getDetails().royaltyRate
            let expiration = listing.getDetails().expiresAfter

            // Add the new listing to the dictionary.
            let oldListing <- self.listings[listingResourceID] <- listing
            // Note that oldListing will always be nil, but we have to handle it.
            destroy oldListing

            // Listing fee
            // let listingFee <- payment.withdraw(amount: Flowty.ListingFee)
            // let flowtyFusdReceiver = Flowty.account.borrow<&FUSD.Vault{FungibleToken.Receiver}>(from: Flowty.FusdVaultStoragePath)
            //     ?? panic("Missing or mis-typed FUSD Reveiver")
            // flowtyFusdReceiver.deposit(from: <-listingFee)
            destroy payment

            let enabledAutoRepayment = fusdProviderCapability != nil

            emit ListingAvailable(
                flowtyStorefrontAddress: self.owner?.address!,
                flowtyStorefrontID: self.uuid,
                listingResourceID: listingResourceID,
                nftType: nftType,
                nftID: nftID,
                amount: amount,
                interestRate: interestRate,
                term: term,
                enabledAutoRepayment: enabledAutoRepayment,
                royaltyRate: royaltyRate,
                expiresAfter: expiration,
                paymentTokenType: paymentVaultType
            )

            return listingResourceID
        }

        // removeListing
        // Remove a Listing that has not yet been funded from the collection and destroy it.
        //
        pub fun removeListing(listingResourceID: UInt64) {
            let listing <- self.listings.remove(key: listingResourceID)
                ?? panic("missing Listing")
    
            // This will emit a ListingCompleted event.
            destroy listing
        }

        // getListingIDs
        // Returns an array of the Listing resource IDs that are in the collection
        //
        pub fun getListingIDs(): [UInt64] {
            return self.listings.keys
        }

        // borrowListing
        // Returns a read-only view of the Listing for the given listingID if it is contained by this collection.
        //
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}? {
            if self.listings[listingResourceID] != nil {
                return &self.listings[listingResourceID] as! &Listing{ListingPublic}
            } else {
                return nil
            }
        }

        // cleanup
        // Remove an listing *if* it has been funded and expired.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        pub fun cleanup(listingResourceID: UInt64) {
            pre {
                self.listings[listingResourceID] != nil: "could not find listing with given id"
            }

            let listing <- self.listings.remove(key: listingResourceID)!
            assert(listing.getDetails().funded == true, message: "listing is not funded, only admin can remove")
            destroy listing
        }

        pub fun getRoyalties(): {String:Flowty.Royalty} {
            return Flowty.Royalties
        }

        // destructor
        //
        destroy () {
            destroy self.listings

            // Let event consumers know that this storefront will no longer exist
            emit FlowtyStorefrontDestroyed(flowtyStorefrontResourceID: self.uuid)
        }

        // constructor
        //
        init () {
            self.listings <- {}

            // Let event consumers know that this storefront exists
            emit FlowtyStorefrontInitialized(flowtyStorefrontResourceID: self.uuid)
        }
    }

    // createStorefront
    // Make creating a FlowtyStorefront publicly accessible.
    //
    pub fun createStorefront(): @FlowtyStorefront {
        return <-create FlowtyStorefront()
    }

    access(account) fun borrowMarketplace(): &Flowty.FlowtyMarketplace {
        return self.account.borrow<&Flowty.FlowtyMarketplace>(from: Flowty.FlowtyMarketplaceStoragePath)
            ?? panic("Missing or mis-typed Flowty FlowtyMarketplace")
    }

    pub fun getRoyalty(nftTypeIdentifier: String): Royalty {
        return Flowty.Royalties[nftTypeIdentifier]!
    }

    pub fun getTokenPaths(): {String:PublicPath} {
        return self.TokenPaths
    }

    // FlowtyAdmin
    // Allows the adminitrator to set the amount of fees, set the suspended funding period
    //
    pub resource FlowtyAdmin {
        pub fun setFees(listingFixedFee: UFix64, fundingPercentageFee: UFix64) {
            pre {
                // The UFix64 type covers a negative numbers
                fundingPercentageFee <= 1.0: "Funding fee should be a percentage"
            }

            Flowty.ListingFee = listingFixedFee
            Flowty.FundingFee = fundingPercentageFee
        }

        pub fun setSuspendedFundingPeriod(period: UFix64) {
            Flowty.SuspendedFundingPeriod = period
        }

        pub fun setSupportedCollection(collection: String, state: Bool) {
            Flowty.SupportedCollections[collection] = state
            emit CollectionSupportChanged(collectionIdentifier: collection, state: state)
        }

        pub fun setCollectionRoyalty(collection: String, royalty: Royalty) {
            pre {
                royalty.Rate <= 1.0: "Royalty rate must be a percentage"
            }

            Flowty.Royalties[collection] = royalty
            emit RoyaltyAdded(collectionIdentifier: collection, rate: royalty.Rate)
        }

        pub fun registerFungibleTokenPath(vaultType: Type, path: PublicPath) {
            Flowty.TokenPaths[vaultType.identifier] = path
        }
    }

    init () {
        self.FlowtyStorefrontStoragePath = /storage/FlowtyStorefront
        self.FlowtyStorefrontPublicPath = /public/FlowtyStorefront
        self.FlowtyMarketplaceStoragePath = /storage/FlowtyMarketplace
        self.FlowtyMarketplacePublicPath = /public/FlowtyMarketplace
        self.FlowtyAdminStoragePath = /storage/FlowtyAdmin
        self.FusdVaultStoragePath = /storage/fusdVault
        self.FusdReceiverPublicPath = /public/fusdReceiver
        self.FusdBalancePublicPath = /public/fusdBalance

        self.ListingFee = 1.0 // Fixed FUSD
        self.FundingFee = 0.1 // Percentage of the interest, a number between 0 and 1.
        self.SuspendedFundingPeriod = 300.0 // Period in seconds
        self.Royalties = {}
        self.SupportedCollections = {}
        self.TokenPaths = {}

        let marketplace <- create FlowtyMarketplace()

        self.account.save(<-marketplace, to: self.FlowtyMarketplaceStoragePath) 
        // create a public capability for the .Marketplace
        self.account.link<&Flowty.FlowtyMarketplace{Flowty.FlowtyMarketplacePublic}>(Flowty.FlowtyMarketplacePublicPath, target: Flowty.FlowtyMarketplaceStoragePath)

        // FlowtyAdmin
        let flowtyAdmin <- create FlowtyAdmin()
        self.account.save(<-flowtyAdmin, to: self.FlowtyAdminStoragePath)

        emit FlowtyInitialized()
    }
}
 