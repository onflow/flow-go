import NonFungibleToken from 0x1d7e57aa55817448
import PonsCertificationContract from 0x856bd81e73e6752b


/*
	Pons NFT Contract Interface

	This smart contract contains the core type requirements for Pons NFTs.
	In the Pons NFT marketplace, properties of the system are enforced by types whenever possible.
	All implementations of Pons NFTs must conform to both this interface, and the NonFungibleToken contract interface.

	As a contract interface, this cannot be merged with the concrete PonsNft contract implementations.
*/
pub contract interface PonsNftContractInterface {

/*
	PonsNft resource interface definition

	Pons NFTs must implement this in addition to the INFT from the NonFungibleToken contract.
	PonsNft resources must contain a ponsCertification to ensure its authenticity of being created by Pons, with the type system.
	PonsNft resources must contain a nftId, a globally unique ID identifying the Pons NFT.
	Usage of the nftId is encouraged, as 1) it is much more difficult to unintentionally specify a nftId, and 2) the nftId is integrated with the Pons App system.
*/
	pub resource interface PonsNft {
		pub ponsCertification : @PonsCertificationContract.PonsCertification
		pub nftId : String }

/*
	PonsCollection resource interface definition

	This is implemented by Pons NFT Collections. 
	PonsCollection resources must contain a ponsCertification to ensure its authenticity of being created by Pons, with the type system.
	Methods are provided for withdraw, deposit, borrow, and viewing the available NFTs, using the nftId as opposed to the id from NonFungibleToken.
*/
	pub resource interface PonsCollection {
		/* Proof of certification from Pons */
		pub ponsCertification : @PonsCertificationContract.PonsCertification

		/* Withdraw a NFT from the PonsCollection, given its nftId */
		pub fun withdrawNft (nftId : String) : @NFT {
			post {
				result .nftId == nftId:
					"Withdrawn Pons NFT does not match the given nftId" } }

		/* Deposit a NFT to the PonsCollection */
		pub fun depositNft (_ ponsNft : @NFT) : Void

		/* Get a list of nftIds stored in the PonsCollection */
		pub fun getNftIds () : [String]

		/* Borrow a reference to a NFT in the PonsCollection */
		pub fun borrowNft (nftId : String) : &NFT {
			post {
				result .nftId == nftId:
					"Borrowed Pons NFT does not match this nftId" } } }

/*
	PonsCollection resource interface definition

	This is implemented by Pons NFT Collections. 
	PonsCollection resources must contain a ponsCertification to ensure its authenticity of being created by Pons, with the type system.
	Methods are provided for withdraw, deposit, borrow, and viewing the available NFTs, using the nftId as opposed to the id from NonFungibleToken.
*/
	pub resource interface PonsNftReceiver {
		/* Proof of certification from Pons */
		access(account) ponsCertification : @PonsCertificationContract.PonsCertification

		/* Deposit a NFT to the PonsCollection */
		pub fun depositNft (_ ponsNft : @NFT) : Void }


	/* All implementing contracts must implement the NFT resource, fulfilling requirements of PonsNft and the requirements from the NonFungibleToken contract */
	pub resource NFT: PonsNft, NonFungibleToken.INFT {}

	/* All implementing contracts must implement the Collection resource, fulfilling requirements of PonsCollection and the requirements from the NonFungibleToken contract */
	pub resource Collection: PonsCollection, PonsNftReceiver, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {} }
