import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import PonsNftContractInterface from 0x26f07529cfd446c3
import PonsUtils from 0x26f07529cfd446c3



/*
	Pons NFT Contract

	This smart contract contains the core functionality of Pons NFTs and definitions for Pons Artists.
	The contract provides APIs to access useful information of Pons NFTs, and defines events which signify the creation and movement of Pons NFTs.
	The contract also provides APIs to access useful information of Pons artists, and defines events which signify recognition of Pons artists.
	This smart contract serves as the API for users of Pons NFTs, and delegates concrete functionality to another resource which implements contract functionality, so that updates can be made to Pons NFTs in a controlled manner if necessary.
*/
pub contract PonsNftContract {

	/* Standardised storage path for PonsCollection */
	pub let CollectionStoragePath : StoragePath

	/* Storage path for PonsArtistAuthority */
	access(account) let ArtistAuthorityStoragePath : StoragePath

	/* Total Pons NFTs minted */
	pub var mintedCount : UInt64

	/* Stores the resource instance to all PonsArtists */
	access(account) var ponsArtists : @{String: PonsArtist}

	/* Stores the ponsArtistId corresponding to each Address */
	access(account) var ponsArtistIds : {Address: String}

	/* Stores the Address corresponding to each ponsArtistId */
	access(account) var addresses : {String: Address}
	
	/* Stores the metadata of each PonsArtist */
	access(account) var metadatas : {String: {String: String}}

	/* Stores the Capability to receive Flow tokens for each artist */
	access(account) var receivePaymentCaps : {String: Capability<&{FungibleToken.Receiver}>}



	/* PonsNftContractInit is emitted on initialisation of this contract */
	pub event PonsNftContractInit ()

	/* PonsNftMinted is emitted on minting of new Pons NFTs */
	pub event PonsNftMinted (nftId : String, serialNumber : UInt64, artistId : String, royalty : PonsUtils.Ratio, editionLabel : String, metadata : {String: String})

	/* PonsNftWithdrawFromCollection is emitted on withdrawal of Pons NFTs from its colllection */
	pub event PonsNftWithdrawFromCollection (nftId : String, serialNumber : UInt64, from : Address?)
	/* PonsNftWithdrawFromCollection is emitted on depositing of Pons NFTs to a colllection */
	pub event PonsNftDepositToCollection (nftId : String, serialNumber : UInt64, to : Address?)

	/* PonsArtistRecognised is emitted on the recognition an artist by Pons on the blockchain */
	pub event PonsArtistRecognised (ponsArtistId : String, metadata : {String: String}, addressOptional : Address?)


	/* Allow the PonsNft events to be emitted by implementations of Pons NFTs from the Pons account */
	access(account) fun emitPonsNftMinted (nftId : String, serialNumber : UInt64, artistId : String, royalty : PonsUtils.Ratio, editionLabel : String, metadata : {String: String}) : Void {
		emit PonsNftMinted (nftId: nftId, serialNumber: serialNumber, artistId: artistId, royalty: royalty, editionLabel: editionLabel, metadata: metadata) }

	access(account) fun emitPonsNftWithdrawFromCollection (nftId : String, serialNumber : UInt64, from : Address?) : Void {
		emit PonsNftWithdrawFromCollection (nftId: nftId, serialNumber: serialNumber, from: from) }
	access(account) fun emitPonsNftDepositToCollection (nftId : String, serialNumber : UInt64, to : Address?) : Void {
		emit PonsNftDepositToCollection (nftId: nftId, serialNumber: serialNumber, to: to) }







	/* Takes the next unused UInt64 as the NonFungibleToken.INFT id; which we call serialNumber */
	access(account) fun takeSerialNumber () : UInt64 {
		self .mintedCount = self .mintedCount + UInt64 (1)

		return self .mintedCount }





	/* Gets the nftId of a Pons NFT */
	pub fun getNftId (_ ponsNftRef : &PonsNftContractInterface.NFT) : String {
		return ponsNftRef .nftId }
	
	/* Gets the serialNumber of a Pons NFT */
	pub fun getSerialNumber (_ ponsNftRef : &PonsNftContractInterface.NFT) : UInt64 {
		return ponsNftRef .id }



	/* Borrows the PonsArtist of a Pons NFT */
	pub fun borrowArtist (_ ponsNftRef : &PonsNftContractInterface.NFT) : &PonsArtist {
		return PonsNftContract .implementation .borrowArtist (ponsNftRef) }

	/* Gets the royalty Ratio of a Pons NFT (i.e. how much percentage of resales are royalties to the artist) */
	pub fun getRoyalty (_ ponsNftRef : &PonsNftContractInterface.NFT) : PonsUtils.Ratio {
		return PonsNftContract .implementation .getRoyalty (ponsNftRef) }

	/* Gets the edition label a Pons NFT to differentiate between distinct limited editions */
	pub fun getEditionLabel (_ ponsNftRef : &PonsNftContractInterface.NFT) : String {
		return PonsNftContract .implementation .getEditionLabel (ponsNftRef) }

	/* Gets any other metadata of a Pons NFT (e.g. IPFS media url) */
	pub fun getMetadata (_ ponsNftRef : &PonsNftContractInterface.NFT) : {String: String} {
		return PonsNftContract .implementation .getMetadata (ponsNftRef) }


	/* API for creating new PonsCollection */
	access(account) fun createEmptyPonsCollection () : @PonsNftContractInterface.Collection {
		return <- PonsNftContract .implementation .createEmptyPonsCollection () }




	/* API to produce an updated Pons NFT from any Pons NFT, so that the Pons contracts can perform any contract updates in a controlled, future-proof manner */
	pub fun updatePonsNft (_ ponsNft : @PonsNftContractInterface.NFT) : @PonsNftContractInterface.NFT {
		return <- PonsNftContract .implementation .updatePonsNft (<- ponsNft) }




/*
	PonsNft implementation resource interface

	This interface defines the concrete functionality that the PonsNft contract delegates.
*/
	pub resource interface PonsNftContractImplementation {
		pub fun borrowArtist (_ ponsNftRef : &PonsNftContractInterface.NFT) : &PonsArtist 
		pub fun getRoyalty (_ ponsNftRef : &PonsNftContractInterface.NFT) : PonsUtils.Ratio 
		pub fun getEditionLabel (_ ponsNftRef : &PonsNftContractInterface.NFT) : String 
		pub fun getMetadata (_ ponsNftRef : &PonsNftContractInterface.NFT) : {String: String} 

		access(account) fun createEmptyPonsCollection () : @PonsNftContractInterface.Collection

		pub fun updatePonsNft (_ ponsNft : @PonsNftContractInterface.NFT) : @PonsNftContractInterface.NFT }

	/* A list recording all previously active instances of PonsNftContractImplementation */
	access(account) var historicalImplementations : @[{PonsNftContractImplementation}]
	/* The currently active instance of PonsNftContractImplementation */
	access(account) var implementation : @{PonsNftContractImplementation}

	/* Updates the currently active PonsNftContractImplementation */
	access(account) fun update (_ newImplementation : @{PonsNftContractImplementation}) : Void {
		var implementation <- newImplementation
		implementation <-> PonsNftContract .implementation
		PonsNftContract .historicalImplementations .append (<- implementation) }



	/* A trivial instance of PonsNftContractImplementation which panics on all calls, used on initialization of the PonsNft contract. */
	pub resource InvalidPonsNftContractImplementation : PonsNftContractImplementation {
		pub fun borrowArtist (_ ponsNftRef : &PonsNftContractInterface.NFT) : &PonsArtist {
			panic ("not implemented") }
		pub fun getRoyalty (_ ponsNftRef : &PonsNftContractInterface.NFT) : PonsUtils.Ratio {
			panic ("not implemented") }
		pub fun getEditionLabel (_ ponsNftRef : &PonsNftContractInterface.NFT) : String {
			panic ("not implemented") }
		pub fun getMetadata (_ ponsNftRef : &PonsNftContractInterface.NFT) : {String: String} {
			panic ("not implemented") }

		access(account) fun createEmptyPonsCollection () : @PonsNftContractInterface.Collection {
			panic ("not implemented") }

		pub fun updatePonsNft (_ ponsNft : @PonsNftContractInterface.NFT) : @PonsNftContractInterface.NFT {
			panic ("not implemented") } }



/*
	Pons Artist Resource

	This resource represents each verified Pons artist.
	For extensibility, concrete artist information is stored outside of the reource (which can be updated), so it only contains the identifying ponsArtistId.
	All PonsArtist resources are kept in the Pons account.
*/
	pub resource PonsArtist {
		pub let ponsArtistId : String

		init (ponsArtistId : String) {
			self .ponsArtistId = ponsArtistId } }

/*
	Pons Artist Certificate Resource

	This resource represents an authorisation from a Pons artist.
	This can be created by the artist himself, or by the Pons account on behalf of the artist.
*/
	pub resource PonsArtistCertificate {
		pub let ponsArtistId : String

		init (ponsArtistId : String) {
			pre {
				PonsNftContract .ponsArtists .containsKey (ponsArtistId):
					"Not recognised Pons Artist" }

			self .ponsArtistId = ponsArtistId } }


	/* Borrow any PonsArtist, given his ponsArtistId, to browse further information about the artist */
	pub fun borrowArtistById (ponsArtistId : String) : &PonsArtist {
		var ponsArtist <- PonsNftContract .ponsArtists .remove (key: ponsArtistId) !
		let ponsArtistRef = & ponsArtist as &PonsArtist
		var replacedArtistOptional <- PonsNftContract .ponsArtists .insert (key: ponsArtistId, <- ponsArtist) 
		destroy replacedArtistOptional
		return ponsArtistRef }


	/* Get the metadata of a PonsArtist */
	pub fun getArtistMetadata (_ ponsArtist : &PonsArtist) : {String: String} {
		return PonsNftContract .metadatas [ponsArtist .ponsArtistId] ! }

	/* Get the Flow address of a PonsArtist if available */
	pub fun getArtistAddress (_ ponsArtist : &PonsArtist) : Address? {
		return PonsNftContract .addresses [ponsArtist .ponsArtistId] }

	/* Get the Capability to receive Flow tokens of a PonsArtist */
	pub fun getArtistReceivePaymentCap (_ ponsArtist : &PonsArtist) : Capability<&{FungibleToken.Receiver}>? {
		return PonsNftContract .receivePaymentCaps [ponsArtist .ponsArtistId] }




	/* Create a PonsArtistCertificate authorisation resource, given his Pons Collection as proof of his identity */
	pub fun makePonsArtistCertificate (_ artistPonsCollectionRef : &PonsNftContractInterface.Collection) : @PonsArtistCertificate {
		pre {
			artistPonsCollectionRef .owner != nil:
				"Please provide a reference to your Pons Collection in your account storage, to prove your identity as a Pons artist"
			PonsNftContract .ponsArtistIds .containsKey (artistPonsCollectionRef .owner! .address):
				"No artist is known to have this address" }
		let ponsArtistId = PonsNftContract .ponsArtistIds [artistPonsCollectionRef .owner! .address] !
		return <- create PonsArtistCertificate (ponsArtistId: ponsArtistId) }



/* 
	Pons Artist Authority Resource

	This resource allows Pons to manage information on Pons artists.
	All Pons artist information can be viewed or modified with a PonsArtistAuthority.
	This resource also allows recognising new Pons artists, and create PonsArtistCertificate authorisations on their behalf.
*/
	pub resource PonsArtistAuthority {
		/* Borrow the dictionary which stores all PonsArtist instances */
		pub fun borrowPonsArtists () : &{String: PonsArtist} {
			return & PonsNftContract .ponsArtists as &{String: PonsArtist} }

		/* Get the dictionary mapping Address to ponsArtistId */
		pub fun getPonsArtistIds () : {Address: String} {
			return PonsNftContract .ponsArtistIds }
		/* Update the dictionary mapping Address to ponsArtistId */
		pub fun setPonsArtistIds (_ ponsArtistIds :  {Address: String}) : Void {
			PonsNftContract .ponsArtistIds = ponsArtistIds }

		/* Get the dictionary mapping ponsArtistId to Address */
		pub fun getAddresses () : {String: Address} {
			return PonsNftContract .addresses }
		/* Update the dictionary mapping ponsArtistId to Address */
		pub fun setAddresses (_ addresses : {String: Address}) : Void {
			PonsNftContract .addresses = addresses }

		/* Get the dictionary mapping ponsArtistId to metadata */
		pub fun getMetadatas () : {String: {String: String}} {
			return PonsNftContract .metadatas }
		/* Update the dictionary mapping ponsArtistId to metadata */
		pub fun setMetadatas (_ metadatas : {String: {String: String}}) : Void {
			PonsNftContract .metadatas = metadatas }

		/* Get the dictionary mapping ponsArtistId to Capability of receiving Flow tokens */
		pub fun getReceivePaymentCaps () : {String: Capability<&{FungibleToken.Receiver}>} {
			return PonsNftContract .receivePaymentCaps }
		/* Update the dictionary mapping ponsArtistId to Capability of receiving Flow tokens */
		pub fun setReceivePaymentCaps (_ receivePaymentCaps : {String: Capability<&{FungibleToken.Receiver}>}) : Void {
			PonsNftContract .receivePaymentCaps = receivePaymentCaps }

		/* Recognise a new Pons artist, and store the PonsArtist resource instance */
		pub fun recognisePonsArtist
		( ponsArtistId : String
		, metadata : {String: String}
		, _ addressOptional : Address?
		, _ receivePaymentCapOptional : Capability<&{FungibleToken.Receiver}>?
		) : Void {
			pre {
				! PonsNftContract .ponsArtists .containsKey (ponsArtistId):
					"Pons Artist with this ponsArtistId already exists" }
			post {
				PonsNftContract .ponsArtists .containsKey (ponsArtistId):
					"Unable to recognise Pons Artist" }

			// Create a PonsArtist with the specified ponsArtistId
			var ponsArtist <- create PonsArtist (ponsArtistId: ponsArtistId)

			// Store the PonsArtist resource into the PonsArtist contract storage
			// Ensure that the key has not been taken
			var replacedArtistOptional <- PonsNftContract .ponsArtists .insert (key: ponsArtistId, <- ponsArtist)
			if replacedArtistOptional != nil {
				panic ("Pons Artist with this ponsArtistId already exists") }
			destroy replacedArtistOptional

			// Save the Pons artist's metadata
			PonsNftContract .metadatas .insert (key: ponsArtistId, metadata)

			// Save the address information of the Pons artist
			if addressOptional != nil {
				PonsNftContract .ponsArtistIds .insert (key: addressOptional !, ponsArtistId)
				PonsNftContract .addresses .insert (key: ponsArtistId, addressOptional !) }

			// Save the artist's Capability to receive Flow tokens
			if receivePaymentCapOptional != nil {
				PonsNftContract .receivePaymentCaps .insert (key: ponsArtistId, receivePaymentCapOptional !) }

			emit PonsArtistRecognised (ponsArtistId: ponsArtistId, metadata: metadata, addressOptional: addressOptional) }

		/* Create a PonsArtistCertificate authorisation, given a PonsArtist reference */
		pub fun makePonsArtistCertificateFromArtistRef (_ ponsArtistRef : &PonsArtist) : @PonsArtistCertificate {
			return <- create PonsArtistCertificate (ponsArtistId: ponsArtistRef .ponsArtistId) }

		/* Create a PonsArtistCertificate authorisation, given a ponsArtistId */
		pub fun makePonsArtistCertificateFromId (ponsArtistId : String) : @PonsArtistCertificate {
			return <- create PonsArtistCertificate (ponsArtistId: ponsArtistId) } }


	init () {
		// Save the standardised Pons collection storage path
		self .CollectionStoragePath = /storage/ponsCollection
		// Save the Artist Authority storage path
		self .ArtistAuthorityStoragePath = /storage/ponsArtistAuthority

		self .mintedCount = 0

		self .historicalImplementations <- []
		// Activate InvalidPonsNftContractImplementation as the active implementation of the Pons NFT system
		self .implementation <- create InvalidPonsNftContractImplementation ()

		self .ponsArtists <- {}
		self .ponsArtistIds = {}
		self .addresses = {}
		self .metadatas = {}
		self .receivePaymentCaps = {}

		// Create and save an Artist Authority resource to the storage path
        	self .account .save (<- create PonsArtistAuthority (), to: /storage/ponsArtistAuthority)

		// Emit the PonsNft contract initialisation event
		emit PonsNftContract.PonsNftContractInit () }

	}
