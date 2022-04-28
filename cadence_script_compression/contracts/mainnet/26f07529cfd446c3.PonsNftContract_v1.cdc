import NonFungibleToken from 0x1d7e57aa55817448
import PonsCertificationContract from 0x26f07529cfd446c3
import PonsNftContractInterface from 0x26f07529cfd446c3
import PonsNftContract from 0x26f07529cfd446c3
import PonsUtils from 0x26f07529cfd446c3



/*
	Pons NFT Contract v1

	This smart contract contains the concrete functionality of Pons NFT v1.
	In the v1 implementation, all Pons NFT information is straightforwardly stored inside the contract, and a Pons NFT minter is created and stored in the Pons account.
*/
pub contract PonsNftContract_v1 : PonsNftContractInterface, NonFungibleToken {

	/* Storage path at which the Minter will be stored */
	access(account) let MinterStoragePath : StoragePath

	/* Capability to the Pons NFT Minter, for convenience of usage */
	access(account) let MinterCapability : Capability<&NftMinter_v1>


	/* PonsNftContractInit_v1 is emitted on initialisation of this contract */
	pub event PonsNftContractInit_v1 ()

	/* ContractInitialized is emitted on initilisation of this contract likewise, this variant satisfies requirements from the NonFungibleToken interface */
	pub event ContractInitialized ()
	/* Withdraw is emitted on withdrawals of Pons NFTs from a PonsCollection, this event satisfying requirements from the NonFungibleToken interface */
	pub event Withdraw (id : UInt64, from : Address?)
	/* Withdraw is emitted on deposits of Pons NFTs to a PonsCollection, this event satisfying requirements from the NonFungibleToken interface */
	pub event Deposit (id : UInt64, to : Address?)




	/* Represents the total number of Pons NFTs (v1) created, satisfying requirements from the NonFungibleToken interface */
	pub var totalSupply : UInt64

	/* Map from nftId to serialNumber */
	access(account) var ponsNftSerialNumbers : {String: UInt64}

	/* Map from serialNumber to nftId */
	access(account) var ponsNftIds : {UInt64: String}

	/* Map from nftId to ponsArtistId */
	access(account) var ponsNftArtistIds : {String: String}

	/* Map from nftId to royalties ratio */
	access(account) var ponsNftRoyalties : {String: PonsUtils.Ratio}

	/* Map from nftId to edition label */
	access(account) var ponsNftEditionLabels : {String: String}

	/* Map from nftId to metadata */
	access(account) var ponsNftMetadatas : {String: {String: String}}


	/* The concrete Pons NFT resource. Striaghtforward implementation of the PonsNft and INFT interfaces */
	pub resource NFT : PonsNftContractInterface.PonsNft, NonFungibleToken.INFT {
		/* Ensures the authenticity of this PonsNft; requirement by PonsNft */
		pub let ponsCertification : @PonsCertificationContract.PonsCertification

		/* Unique identifier for the NFT; requirement by PonsNft */
		pub let nftId : String

		/* Unique identifier for the NFT; requirement by INFT */
		pub let id : UInt64

		init (nftId : String, serialNumber : UInt64) {
			pre {
				! PonsNftContract_v1 .ponsNftSerialNumbers .containsKey (nftId): 
					"Pons NFT with this nftId already taken"
				! PonsNftContract_v1 .ponsNftIds .containsKey (serialNumber): 
					"Pons NFT with this serialNumber already taken" }
			post {
				PonsNftContract_v1 .ponsNftSerialNumbers .containsKey (nftId): 
					"Failed to create Pons NFT with this nftId"
				PonsNftContract_v1 .ponsNftIds .containsKey (serialNumber): 
					"Failed to create Pons NFT with this serialNumber" }

			/* Proof of creation by Pons */
			self .ponsCertification <- PonsCertificationContract .makePonsCertification ()
			self .nftId = nftId
			self .id = serialNumber

			/* For the sake of consistency, the link between nftId and serialNumber should be established once it is known */
			PonsNftContract_v1 .ponsNftSerialNumbers .insert (key: nftId, serialNumber)
			PonsNftContract_v1 .ponsNftIds .insert (key: serialNumber, nftId) }

		destroy () {
			destroy self .ponsCertification } }

	/* The concrete Pons Collection resource. Striaghtforward implementation of the PonsCollection, Provider, Receiver, and Collection interfaces */
	pub resource Collection : PonsNftContractInterface.PonsCollection, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

		/* Ensures the authenticity of this PonsCollection; requirement from PonsCollection */
		pub let ponsCertification : @PonsCertificationContract.PonsCertification

		/* Stores the owned NFT resources; requirement from NonFungibleToken.Collection */
		pub var ownedNFTs : @{UInt64: NonFungibleToken.NFT}


		/* Withdraw an NFT from the PonsCollection, given its nftId */
		pub fun withdrawNft (nftId : String) : @PonsNftContractInterface.NFT {
			pre {
				self .ownedNFTs .containsKey (PonsNftContract_v1 .ponsNftSerialNumbers [nftId] !):
					"Pons NFT with this nftId not found" }
			post {
				! self .ownedNFTs .containsKey (PonsNftContract_v1 .ponsNftSerialNumbers [nftId] !):
					"Failed to withdraw Pons NFT with this nftId" }

			// Find the serialNumber of the NFT given its nftId
			let serialNumber = PonsNftContract_v1 .ponsNftSerialNumbers [nftId] !

			// Retrieve the NFT by removing it from the collection's ownedNFTs
			// We know it is a PonsNftContractInterface.NFT, and we cast it
			var ponsNft : @PonsNftContractInterface.NFT <-
				(self .ownedNFTs .remove (key: serialNumber) ! as! @PonsNftContractInterface.NFT)

			// Emit the PonsNft_v1 Withdraw event (due to the NonFungibleToken contract)
			emit Withdraw (id: serialNumber, from: self .owner ?.address)
			// Emit the PonsNft Withdraw event
            		PonsNftContract .emitPonsNftWithdrawFromCollection (nftId: nftId, serialNumber: ponsNft .id, from: self .owner ?.address)

			// return the PonsNft, with the latest updates
			return <- PonsNftContract .updatePonsNft (<- ponsNft) }

		/* Deposit a NFT from the PonsCollection, given its nftId */
		pub fun depositNft (_ ponsNft : @PonsNftContractInterface.NFT) : Void {
			pre {
				! self .ownedNFTs .containsKey (PonsNftContract_v1 .ponsNftSerialNumbers [ponsNft .nftId] !):
					"Pons NFT with this nftId already in this collection" }
			/*post {
				self .ownedNFTs .containsKey (PonsNftContract_v1 .ponsNftSerialNumbers [before (ponsNft .nftId)] !):
					"" }*/

			let nftId = ponsNft .nftId
			let serialNumber = ponsNft .id

			// All NFT resources in contracts implementing the PonsNftContractInterface should also implement NonFungibleToken
			var nft <- ponsNft as! @NonFungibleToken.NFT

			// Insert the NFT into the collection if the key is not taken
			var replacedTokenOptional <-
				self .ownedNFTs .insert (key: serialNumber, <- nft)
			if replacedTokenOptional != nil {
				panic ("Aborting transaction due to conflict with another token") }
			// Satisfy the resource checker
			destroy replacedTokenOptional

			// Emit the PonsNft_v1 Deposit event (due to the NonFungibleToken contract)
			emit Deposit (id: serialNumber, to: self .owner ?.address)
			// Emit the PonsNft Deposit event
            		PonsNftContract .emitPonsNftDepositToCollection (nftId: nftId, serialNumber: serialNumber, to: self .owner ?.address) }

		/* Get a list of nftIds stored in the PonsCollection */
		pub fun getNftIds () : [String] {
			// Iterate over the keys of ownedNFTs, convert them to nftIds, and put them in an array
			let serialNumbers = self .ownedNFTs .keys
			var nftIds : [String] = []
			var index = 0
			while index < serialNumbers .length {
				let serialNumber = serialNumbers [index] !
				let nftId = PonsNftContract_v1 .ponsNftIds [serialNumber] !
				nftIds .append (nftId)
				index = index + 1 }
			return nftIds }

		/* Borrow a reference to a NFT in the PonsCollection, given its nftId */
		pub fun borrowNft (nftId : String) : &PonsNftContractInterface.NFT {
			pre {
				self .ownedNFTs .containsKey (PonsNftContract_v1 .ponsNftSerialNumbers [nftId] !):
					"Pons NFT with this nftId not found" }

			let serialNumber = PonsNftContract_v1 .ponsNftSerialNumbers [nftId] !

			// First, remove the NFT from ownedNFTs
			var nft <- self .ownedNFTs .remove (key: serialNumber) ! as! @PonsNftContractInterface.NFT
			// Then, update it
			var updatedNft <- PonsNftContract .updatePonsNft (<- nft)
			// Then, insert it back to the same key
			var nilNft <- self .ownedNFTs .insert (key: serialNumber, <- (updatedNft as! @NonFungibleToken.NFT))
			// The insert function returns a @NFT? representing the NFT previously at the key, which we just removed the NFT from, so it is nil
			// Destroy the nil to make the resource checker happy
			destroy nilNft

			// Get a reference to the NFT at the corresponding serialNumber, and cast it to the return type
			let nftRef = & self .ownedNFTs [serialNumber] as auth &NonFungibleToken.NFT
			let ponsNftRef = nftRef as! &PonsNftContractInterface.NFT
            		return ponsNftRef }



		/* Withdraw an NFT from the PonsCollection, given its serialNumber */
		pub fun withdraw (withdrawID : UInt64) : @NonFungibleToken.NFT {
			let nftId = PonsNftContract_v1 .ponsNftIds [withdrawID] !
			var nft <- self .withdrawNft (nftId: nftId) as! @NonFungibleToken.NFT
			return <- nft }

		/* Deposit an NFT to the PonsCollection */
		pub fun deposit (token : @NonFungibleToken.NFT) : Void {
			var nft <- token as! @PonsNftContractInterface.NFT
			self .depositNft (<- nft) }

		/* Get a list of serialNumbers stored in the PonsCollection */
		pub fun getIDs () : [UInt64] {
			return self .ownedNFTs .keys }

		/* Borrow a reference to a NFT in the PonsCollection, given its serialNumber */
		pub fun borrowNFT (id : UInt64) : &NonFungibleToken.NFT {
			pre {
				self .ownedNFTs .containsKey (id):
					"Pons NFT with this serialNumber not found" }

			let serialNumber = id

			// First, remove the NFT from ownedNFTs
			var nft <- self .ownedNFTs .remove (key: serialNumber) ! as! @PonsNftContractInterface.NFT
			// Then, update it
			var updatedNft <- PonsNftContract .updatePonsNft (<- nft)
			// Then, insert it back to the same key
			var nilNft <- self .ownedNFTs .insert (key: serialNumber, <- (updatedNft as! @NonFungibleToken.NFT))
			// The insert function returns a @NFT? representing the NFT previously at the key, which we just removed the NFT from, so it is nil
			// Destroy the nil to make the resource checker happy
			destroy nilNft

			// Get a reference to the NFT at the corresponding serialNumber, and cast it to the return type
			let nftRef = & self .ownedNFTs [serialNumber] as &NonFungibleToken.NFT
			return nftRef }



		init () {
			self .ponsCertification <- PonsCertificationContract .makePonsCertification ()
			self .ownedNFTs <- {} }

		destroy () {
			// Discourage the manipulation of Pons collections for purposes other than storage of Pons NFTs
			if self .owner ?.address != PonsNftContract_v1 .account .address {
				panic ("Pons Collections cannot be destroyed") }

			destroy self .ponsCertification
			destroy self .ownedNFTs } }

	/* Create a empty Pons Collection */
	pub fun createEmptyCollection () : @Collection {
		return <- create Collection () }




/*
	Pons NFT v1 Minter resource

	This resource enables the minting of new Pons NFTs.
	All Pons NFTs have an nftId, which is handed out by the minter.
	As the blockchain is ill-suited to generating IDs which have randomness and correspond to data on the Pons system, the owner of the resource can provide the available nftIds, using the refillMintIds function.
*/
	pub resource NftMinter_v1 {
		/* Stores available nftIds for new new NFTs */
		access(account) var nftIds : [String]

		/* Adds nftIds to the pool of available nftIds */
		pub fun refillMintIds (mintIds : [String]) {
			var idIndex = 0
			while idIndex < mintIds .length {
				let mintId = mintIds [idIndex]
				if PonsNftContract_v1 .ponsNftSerialNumbers .containsKey (mintId) {
					panic ("The nftId " .concat (mintId) .concat (" has already been taken")) }
				if self .nftIds .contains (mintId) {
					panic ("The nftId " .concat (mintId) .concat (" has already been taken")) }
				idIndex = idIndex + 1 }
			self .nftIds .appendAll (mintIds) }

		/* Mints a new Pons NFT v1 */
		pub fun mintNft
		( _ artistCertificate : &PonsNftContract.PonsArtistCertificate
		, royalty : PonsUtils.Ratio
		, editionLabel : String
		, metadata : {String: String}
		) : @PonsNftContractInterface.NFT {
			pre {
				self .nftIds .length > 0:
					"Pons NFT Minter out of nftIds" }

			// Takes the next available nftId
			let nftId = self .nftIds .remove (at: 0) !
			// Takes the next available serialNumber
			let serialNumber = PonsNftContract .takeSerialNumber ()

			var nft <- create NFT (nftId: nftId, serialNumber: serialNumber)
			let nftRef = & nft as &PonsNftContractInterface.NFT

			// Associate the specified data with the NFT
			PonsNftContract_v1 .ponsNftArtistIds .insert (key: nftId, artistCertificate .ponsArtistId)
			PonsNftContract_v1 .ponsNftRoyalties .insert (key: nftId, royalty)
			PonsNftContract_v1 .ponsNftEditionLabels .insert (key: nftId, editionLabel)
			PonsNftContract_v1 .ponsNftMetadatas .insert (key: nftId, metadata)

			// Increment the totalSupply of Pons NFT v1
			PonsNftContract_v1 .totalSupply = PonsNftContract_v1 .totalSupply + UInt64 (1)

			// Emit the PonsNft Minted event
			PonsNftContract .emitPonsNftMinted (
				nftId: nft .nftId,
				serialNumber: nft .id,
				artistId: PonsNftContract .borrowArtist (nftRef) .ponsArtistId,
				royalty : PonsNftContract .getRoyalty (nftRef),
				editionLabel : PonsNftContract .getEditionLabel (nftRef),
				metadata : PonsNftContract .getMetadata (nftRef) )

			return <- nft }

		init () {
			self .nftIds = [] } }




	/* A straightforward instance of PonsNftContractImplementation which utilises PonsNftContract_v1 for all its functionality, used on initialization of the PonsNftContract_v1 contract. */
	pub resource PonsNftContractImplementation_v1 : PonsNftContract.PonsNftContractImplementation {
		pub fun borrowArtist (_ ponsNftRef : &PonsNftContractInterface.NFT) : &PonsNftContract.PonsArtist {
			let ponsArtistId = PonsNftContract_v1 .ponsNftArtistIds [ponsNftRef .nftId] !
			return PonsNftContract .borrowArtistById (ponsArtistId: ponsArtistId) }
		pub fun getRoyalty (_ ponsNftRef : &PonsNftContractInterface.NFT) : PonsUtils.Ratio {
			return PonsNftContract_v1 .ponsNftRoyalties [ponsNftRef .nftId] ! }
		pub fun getEditionLabel (_ ponsNftRef : &PonsNftContractInterface.NFT) : String {
			return PonsNftContract_v1 .ponsNftEditionLabels [ponsNftRef .nftId] ! }
		pub fun getMetadata (_ ponsNftRef : &PonsNftContractInterface.NFT) : {String: String} {
			return PonsNftContract_v1 .ponsNftMetadatas [ponsNftRef .nftId] ! }

		access(account) fun createEmptyPonsCollection () : @PonsNftContractInterface.Collection {
			return <- create Collection () }

		pub fun updatePonsNft (_ ponsNft : @PonsNftContractInterface.NFT) : @PonsNftContractInterface.NFT {
			return <- ponsNft } }


	

	init () {
		// Save the minter storage path
		self .MinterStoragePath = /storage/ponsMinter

		self .totalSupply = 0

		self .ponsNftSerialNumbers = {}
		self .ponsNftIds = {}
		self .ponsNftArtistIds = {}
		self .ponsNftRoyalties = {}
		self .ponsNftEditionLabels = {}
		self .ponsNftMetadatas = {}

		// Save a NFT v1 Minter to the specified storage path
        	self .account .save (<- create NftMinter_v1 (), to: self .MinterStoragePath)

		// Create and save a capability to the minter for convenience
		self .MinterCapability = self .account .link <&NftMinter_v1> (/private/ponsMinter, target: self .MinterStoragePath) !

		// Activate PonsNftContractImplementation_v1 as the active implementation of the Pons NFT system
		PonsNftContract .update (<- create PonsNftContractImplementation_v1 ())

		// Emit the Pons NFT v1 contract initialisation event, required by NonFungibleToken
		emit ContractInitialized ()
		// Emit the Pons NFT v1 contract initialisation event
		emit PonsNftContractInit_v1 () } }
