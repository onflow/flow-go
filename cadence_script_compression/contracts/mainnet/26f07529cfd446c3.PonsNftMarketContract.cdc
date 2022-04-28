import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61
import PonsNftContractInterface from 0x26f07529cfd446c3
import PonsNftContract from 0x26f07529cfd446c3
import PonsUtils from 0x26f07529cfd446c3


/*
	Pons NFT Market Contract

	This smart contract contains the core functionality of the Pons NFT market.
	The contract defines the core mechanisms of minting, listing, purchasing, and unlisting NFTs, and also the Listing Certificate resource that proves a NFT listing.
	This smart contract serves as the API for users of the Pons NFT marketplace, and delegates concrete functionality to another resource which implements contract functionality, so that updates can be made to the marketplace in a controlled manner if necessary.
	When the Pons marketplace mints multiple editions of NFTs, the market price of each succeesive NFT is incremented by an incremental price.
	This is adjustable by the minting Pons artist.
*/
pub contract PonsNftMarketContract {

	/* The storage path for the PonsNftMarket */
	pub let PonsNftMarketAddress : Address
	/* Standardised storage path for PonsListingCertificateCollection */
	pub let PonsListingCertificateCollectionStoragePath : StoragePath


	/* PonsMarketContractInit is emitted on initialisation of this contract */
	pub event PonsMarketContractInit ()

	/* PonsNFTListed is emitted on the listing of Pons NFTs on the marketplace */
	pub event PonsNFTListed (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits)

	/* PonsNFTUnlisted is emitted on the unlisting of Pons NFTs from the marketplace */
	pub event PonsNFTUnlisted (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits)

	/* PonsNFTSold is emitted when a Pons NFT is sold */
	pub event PonsNFTSold (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits)

	/* PonsNFTSold is emitted when a Pons NFT is sold, and the new owner address is known */
	pub event PonsNFTOwns (owner : Address, nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits)


	/* Allow the PonsNft events to be emitted by all implementations of Pons NFTs from the same account */
	access(account) fun emitPonsNFTListed (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits) : Void {
		emit PonsNFTListed (nftId: nftId, serialNumber: serialNumber, editionLabel: editionLabel, price: price) }
	access(account) fun emitPonsNFTUnlisted (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits) : Void {
		emit PonsNFTUnlisted (nftId: nftId, serialNumber: serialNumber, editionLabel: editionLabel, price: price) }
	access(account) fun emitPonsNFTSold (nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits) : Void {
		emit PonsNFTSold (nftId: nftId, serialNumber: serialNumber, editionLabel: editionLabel, price: price) }
	access(account) fun emitPonsNFTOwns (owner : Address, nftId : String, serialNumber : UInt64, editionLabel : String, price : PonsUtils.FlowUnits) : Void {
		emit PonsNFTOwns (owner: owner, nftId: nftId, serialNumber: serialNumber, editionLabel: editionLabel, price: price) }



/*
	Pons NFT Market Resource Interface

	This resource interface defines the mechanisms and requirements for Pons NFT market implementations.
*/
	pub resource interface PonsNftMarket {
		/* Get the nftIds of all NFTs for sale */
		pub fun getForSaleIds () : [String]

		/* Get the price of an NFT */
		pub fun getPrice (nftId : String) : PonsUtils.FlowUnits?

		/* Borrow an NFT from the marketplace, to browse its details */
		pub fun borrowNft (nftId : String) : &PonsNftContractInterface.NFT?

		/* Given a Pons artist certificate, mint new Pons NFTs on behalf of the artist and list it on the marketplace for sale */
		/* The price of the first edition of the NFT minted is determined by the basePrice */
		/* When only one edition is minted, the incrementalPrice is inconsequential */
		/* When the Pons marketplace mints multiple editions of NFTs, the market price of each succeesive NFT is incremented by the incrementalPrice */
		pub fun mintForSale
		( _ artistCertificate : &PonsNftContract.PonsArtistCertificate
		, metadata : {String: String}
		, quantity : Int
		, basePrice : PonsUtils.FlowUnits
		, incrementalPrice : PonsUtils.FlowUnits
		, _ royaltyRatio : PonsUtils.Ratio
		, _ receivePaymentCap : Capability<&{FungibleToken.Receiver}>
		) : @[{PonsListingCertificate}] {
			pre {
				quantity >= 0:
					"The quantity minted must not be a negative number"
				basePrice .flowAmount >= 0.0:
					"The base price must be a positive amount of Flow units"
				incrementalPrice .flowAmount >= 0.0:
					"The base price must be a positive amount of Flow units"
				royaltyRatio .amount >= 0.0:
					"The royalty ratio must be in the range 0% - 100%"
				royaltyRatio .amount <= 1.0:
					"The royalty ratio must be in the range 0% - 100%" }
			/*
			// For some reason not understood, the certificatesOwnedByMarket function fails to type-check in this post-condition
			post {
				PonsNftMarketContract .certificatesOwnedByMarket (& result as &[{PonsListingCertificate}]):
					"Failed to mint NFTs for sale" } */ }

		/* List a Pons NFT on the marketplace for sale */
		pub fun listForSale (_ nft : @PonsNftContractInterface.NFT, _ salePrice : PonsUtils.FlowUnits, _ receivePaymentCap : Capability<&{FungibleToken.Receiver}>) : @{PonsListingCertificate} /*{
			// WORKAROUND -- ignore
			// Flow implementation seems to be inconsistent regarding owners of nested resources
			// https://github.com/onflow/cadence/issues/1320
			post {
				result .listerAddress == before (nft .owner !.address):
					"Failed to list this Pons NFT" } }*/

		/* Purchase a Pons NFT from the marketplace */
		pub fun purchase (nftId : String, _ purchaseVault : @FungibleToken.Vault) : @PonsNftContractInterface.NFT {
			pre {
				// Given that the purchaseVault is a FlowToken vault, preconditions on FungibleToken and FlowToken ensure that
				// the balance of the vault is positive, and that only amounts between zero and the balance of the vault can be withdrawn from the vault, so that
				// attempts to game the market using unreasonable royalty ratios (e.g. < 0% or > 100%) will result in failed assertions
				purchaseVault .isInstance (Type<@FlowToken.Vault> ()):
					"Pons NFTs must be purchased using Flow tokens"
				self .borrowNft (nftId: nftId) != nil:
					"This Pons NFT is not on the market anymore" }
			post {
				result .nftId == nftId:
					"Failed to purchase the Pons NFT" } }
		/* Unlist a Pons NFT from the marketplace */
		pub fun unlist (_ ponsListingCertificate : @{PonsListingCertificate}) : @PonsNftContractInterface.NFT {
			pre {
				// WORKAROUND -- ignore
				/*
				// Flow implementation seems to be inconsistent regarding owners of nested resources
				// https://github.com/onflow/cadence/issues/1320
				// For the moment, allow all listing certificate holders redeem...
				ponsListingCertificate .listerAddress == ponsListingCertificate .owner !.address:
					"Only the lister can redeem his Pons NFT"
				*/
				self .borrowNft (nftId: ponsListingCertificate .nftId) != nil:
					"This Pons NFT is not on the market anymore" } } }
/*
	Pons Listing Certificate Resource Interface

	This resource interface defines basic information about listing certificates.
	Pons market implementations may provide additional details regarding the listing.
*/
	pub resource interface PonsListingCertificate {
		pub listerAddress : Address
		pub nftId : String }

/*
	Pons Listing Certificate Collection Resource

	This resource manages a user's listing certificates, and is stored in a standardised location.
*/
	pub resource PonsListingCertificateCollection {
		pub var listingCertificates : @[{PonsListingCertificate}]

		init () {
			self .listingCertificates <- [] }
		destroy () {
			destroy self .listingCertificates } }

	pub fun createPonsListingCertificateCollection () : @PonsListingCertificateCollection {
		return <- create PonsListingCertificateCollection () }

	/* Checks whether all the listing certificates provided belong to the market */
//	pub fun certificatesOwnedByMarket (_ listingCertificatesRef : &[{PonsListingCertificate}]) : Bool {
//		var index = 0
//		while index < listingCertificatesRef .length {
//			if listingCertificatesRef [index] .listerAddress != PonsNftMarketContract .PonsNftMarketAddress {
//				return false }
//			index = index + 1 }
//		return true }



	/* API to get the nftIds on the market for sale */
	pub fun getForSaleIds () : [String] {
		return PonsNftMarketContract .ponsMarket .getForSaleIds () }

	/* API to get the price of an NFT on the market */
	pub fun getPrice (nftId : String) : PonsUtils.FlowUnits? {
		return PonsNftMarketContract .ponsMarket .getPrice (nftId: nftId) }

	/* API to borrow an NFT for browsing */
	pub fun borrowNft (nftId : String) : &PonsNftContractInterface.NFT? {
		return PonsNftMarketContract .ponsMarket .borrowNft (nftId: nftId) }


	/* API to borrow the active Pons market instance */
	pub fun borrowPonsMarket () : &{PonsNftMarket} {
		return & self .ponsMarket as &{PonsNftMarket} }




	/* A list recording all previously active instances of PonsNftMarket */
	access(account) var historicalPonsMarkets : @[{PonsNftMarket}]
	/* The currently active instance of PonsNftMarket */
	access(account) var ponsMarket : @{PonsNftMarket}

	/* Updates the currently active PonsNftMarket */
	access(account) fun setPonsMarket (_ ponsMarket : @{PonsNftMarket}) : Void {
		var newPonsMarket <- ponsMarket
		newPonsMarket <-> PonsNftMarketContract .ponsMarket
		PonsNftMarketContract .historicalPonsMarkets .append (<- newPonsMarket) }





	init () {
		self .historicalPonsMarkets <- []
		// Activate InvalidPonsNftMarket as the active implementation of the Pons NFT market
		self .ponsMarket <- create InvalidPonsNftMarket ()

		// Save the market address
		self .PonsNftMarketAddress = self .account .address
		// Save the standardised Pons listing certificate collection storage path
		self .PonsListingCertificateCollectionStoragePath = /storage/listingCertificateCollection

		// Emit the PonsNftMarket initialisation event
		emit PonsMarketContractInit () }

	/* An trivial instance of PonsNftMarket which panics on all calls, used on initialization of the PonsNftMarket contract. */
	pub resource InvalidPonsNftMarket : PonsNftMarket {
		pub fun getForSaleIds () : [String] {
			panic ("not implemented") }
		pub fun getPrice (nftId : String) : PonsUtils.FlowUnits? {
			panic ("not implemented") }
		pub fun borrowNft (nftId : String) : &PonsNftContractInterface.NFT? {
			panic ("not implemented") }

		pub fun mintForSale 
		( _ artistCertificate : &PonsNftContract.PonsArtistCertificate
		, metadata : {String: String}
		, quantity : Int
		, basePrice : PonsUtils.FlowUnits
		, incrementalPrice : PonsUtils.FlowUnits
		, _ royaltyRatio : PonsUtils.Ratio
		, _ receivePaymentCap : Capability<&{FungibleToken.Receiver}>
		) : @[{PonsListingCertificate}] {
			panic ("not implemented") }
		pub fun listForSale (_ nft : @PonsNftContractInterface.NFT, _ salePrice : PonsUtils.FlowUnits, _ receivePaymentCap : Capability<&{FungibleToken.Receiver}>) : @{PonsListingCertificate} {
			panic ("not implemented") }
		pub fun purchase (nftId : String, _ purchaseVault : @FungibleToken.Vault) : @PonsNftContractInterface.NFT {
			panic ("not implemented") }
		pub fun purchaseBySerialId (nftSerialId : UInt64, _ purchaseVault : @FungibleToken.Vault) : @PonsNftContractInterface.NFT {
			panic ("not implemented") }
		pub fun unlist (_ ponsListingCertificate : @{PonsListingCertificate}) : @PonsNftContractInterface.NFT {
			panic ("not implemented") } }

	 }
