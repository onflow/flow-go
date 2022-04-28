import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import PonsNftContractInterface from 0x856bd81e73e6752b
import PonsUtils from 0x856bd81e73e6752b


/*
	Pons NFT Market Escrow Contract

	This smart contract contains the Pons Escrow system.
	In the Pons Escrow system, users can submit an Escrow, locking up an amount of their Flow tokens and Pons NFTs for the fulfillment of a certain condition.
	The escrow can be consummated by the Pons account, or alternatively, both the owner of the escrow and the Pons account can terminate the escrow, and release the held resources.
*/
pub contract PonsEscrowContract {

	/* Map from ID to Escrow Capability */
	access(account) let escrowCaps : {String: Capability<&Escrow>}


	/* PonsEscrowContractInit is emitted on initialisation of this contract */
	pub event PonsEscrowContractInit ()

	/* PonsEscrowSubmitted is emitted on submission of a new Escrow */
	pub event PonsEscrowSubmitted (id : String, address : Address, heldResourceDescription : EscrowResourceDescription, requirement : EscrowResourceDescription)

	/* PonsEscrowConsummated is emitted on consummation of an Escrow */
	pub event PonsEscrowConsummated (id : String, address : Address, heldResourceDescription : EscrowResourceDescription, requirement : EscrowResourceDescription, fulfilledResourceDescription : EscrowResourceDescription)

	/* PonsEscrowConsummated is emitted on termination of an Escrow */
	pub event PonsEscrowTerminated (id : String, address : Address, heldResourceDescription : EscrowResourceDescription, requirement : EscrowResourceDescription)

	/* PonsEscrowConsummated is emitted on dismissal of an Escrow */
	pub event PonsEscrowDismissed (id : String, address : Address)


	/* The Escrow Resource resource. Stores resources locked up for an Escrow. */
	pub resource EscrowResource {
		/* Stores locked-up Flow tokens */
		access(account) var flowVault : @FungibleToken.Vault
		/* Stores locked-up Pons NFTs */
		access(account) var ponsNfts : @[PonsNftContractInterface.NFT]

		/* Offer access to the enclosed Flow Token Vault */
		pub fun borrowFlowVault () : &FungibleToken.Vault {
			return & self .flowVault as &FungibleToken.Vault }

		/* Offer access to the enclosed Pons NFT List */
		pub fun borrowPonsNfts () : &[PonsNftContractInterface.NFT] {
			return & self .ponsNfts as &[PonsNftContractInterface.NFT] }

		init (flowVault : @FungibleToken.Vault, ponsNfts : @[PonsNftContractInterface.NFT]) {
			pre {
				flowVault .isInstance (Type<@FlowToken.Vault> ()):
					"Only Flow tokens and Pons NFTs are accepted in EscrowResource" }
			self .flowVault <- flowVault
			self .ponsNfts <- ponsNfts }

		destroy () {
			if self .flowVault .balance != 0.0 {
				panic ("Non-empty EscrowResource cannot be destroyed") }
			if self .ponsNfts .length != 0 {
				panic ("Non-empty EscrowResource cannot be destroyed") }
			destroy self .flowVault
			destroy self .ponsNfts } }
	/* Escrow ResourceDescription struct. Represents requirements for the fulfillment of an Escrow */
	pub struct EscrowResourceDescription {
		/* Represents the amount of Flow tokens needed to consummate an Escrow */
		pub let flowUnits : PonsUtils.FlowUnits
		/* Represents a list of nftIds, of which Pons NFTs are needed to consummate an Escrow */
		access(self) let ponsNftIds : [String]

		/* Allow the required Pons NFT List to be read */
		pub fun getPonsNftIds () : [String] {
			return self .ponsNftIds .concat ([]) }

		init (flowUnits : PonsUtils.FlowUnits, ponsNftIds : [String]) {
			self .flowUnits = flowUnits
			self .ponsNftIds = ponsNftIds } }
	/* Escrow Fulfillment struct. Represents fulfillment capabilities for an Escrow */
	pub struct EscrowFulfillment {
		/* Represents the Capability for receiving demanded Flow tokens of an Escrow */
		pub let receivePaymentCap : Capability<&{FungibleToken.Receiver}>
		/* Represents the Capability for receiving demanded Pons NFTs of an Escrow */
		pub let receiveNftCap : Capability<&{PonsNftContractInterface.PonsNftReceiver}>

		init (receivePaymentCap : Capability<&{FungibleToken.Receiver}>, receiveNftCap : Capability<&{PonsNftContractInterface.PonsNftReceiver}>) {
			self .receivePaymentCap = receivePaymentCap
			self .receiveNftCap = receiveNftCap } }


	pub fun makeEscrowResource (flowVault : @FungibleToken.Vault, ponsNfts : @[PonsNftContractInterface.NFT]) : @EscrowResource {
		return <- create EscrowResource (flowVault: <- flowVault, ponsNfts: <- ponsNfts) }


/*
	Escrow Resource

	This resource defines the Escrow, identified by an id, holding heldResource for a fulfillment with some requirement.
	The id of an Escrow allows the Pons system to consummate specific Escrows with otherwise specified requirements.
*/
	pub resource Escrow {
		/* Information of an Escrow is available to any with access to the resource */
		pub let id : String
		pub let heldResourceDescription : EscrowResourceDescription
		pub let requirement : EscrowResourceDescription
		pub let fulfillment : EscrowFulfillment

		/* Access to the heldResource is not available to any code out of the Escrow, regardless of the owner of the resource */
		access(self) var heldResource : @EscrowResource?

		/* Checks whether an Escrow's resources have been released, which should only happen if the Escrow is either consummated or terminated */
		pub fun isReleased () : Bool {
			return self .heldResource == nil }

		/* Upon consummation, the heldResource is exchanged for another EscrowResource resource that fulfills the Escrow requirement, which is then released via the Escrow fulfillment. */
		access(account) fun consummate (_ consummation : ((@EscrowResource): @EscrowResource)) : EscrowResourceDescription {
			// Check that the Escrow's resources have not yet been released
			if self .isReleased () {
				panic ("The Escrow has already been consummated or terminated, and cannot be consummated again") }

			// Withdraw the resources held in the Escrow
			var resourcesOptional : @EscrowResource? <- self .heldResource <- nil
			var resources <- resourcesOptional !

			// Use the provided consummation function to exchange the held resources for other resources
			var consummationResource <- consummation (<- resources)

			// Check that the obtained resources fulfill the Escrow requirements
			if ! PonsEscrowContract .satisfiesResourceDescription (
				& consummationResource as &EscrowResource,
				self .requirement
			) {
				panic ("The proposed consummation of the Escrow does not fulfill the fulfillment") }

			// Record the concrete resources obtained in consummating the Escrow
			let fulfilledResourceDescription = PonsEscrowContract .resourceDescription (& consummationResource as &EscrowResource)

			// Transfer the obtained resources to the Escrow's fulfillment
			PonsEscrowContract .fullfillResource (<- consummationResource, self .fulfillment)

			return fulfilledResourceDescription }

		/* Upon termination, the Escrow heldResource are directly released to the Escrow fulfillment. */
		access(account) fun terminate () : Void {
			// Check that the Escrow's resources have not yet been released
			if self .isReleased () {
				panic ("The Escrow has already been consummated or terminated, and cannot be terminated again") }

			// Withdraw the resources held in the Escrow, and transfer the obtained resources to the Escrow's fulfillment
			var resources : @EscrowResource? <- nil
			resources <-> self .heldResource
			PonsEscrowContract .fullfillResource (<- resources !, self .fulfillment) }

		init (id : String, resources : @EscrowResource, requirement : EscrowResourceDescription, fulfillment : EscrowFulfillment) {
			self .id = id

			self .heldResourceDescription = PonsEscrowContract .resourceDescription (& resources as &EscrowResource)

			self .heldResource <- resources
			self .requirement = requirement
			self .fulfillment = fulfillment }

		/* Escrows cannot be destroyed without consummation or termination. */
		destroy () {
			// Check that the Escrow's resources have already been released
			if ! self .isReleased () {
				panic ("The Escrow must be consummated or terminated before it can be destroyed") }
			destroy self .heldResource } }

	pub resource EscrowManager {

		/* Gets a reference to an active Escrow with the specified id */
		pub fun escrow (id : String) : &Escrow? {
			let escrowCapOptional = PonsEscrowContract .escrowCaps [id] 
			if escrowCapOptional == nil {
				return nil }
			else {
				return escrowCapOptional !.borrow () } }

		/* Consummate the Escrow with the specified id and consummation function */
		pub fun consummateEscrow (id : String, consummation : ((@EscrowResource): @EscrowResource)) : Void {
			return PonsEscrowContract .consummateEscrow (id: id, consummation: consummation) }

		/* Terminate the Escrow with the specified id */
		pub fun terminateEscrow (id : String) : Void {
			return PonsEscrowContract .terminateEscrow (PonsEscrowContract .escrowCaps [id] !.borrow () !) }

		/* Dismiss the Escrow with the specified id */
		pub fun dismissEscrow (id : String) : Void {
			return PonsEscrowContract .dismissEscrow (id: id) } }



	/* API to submit an Escrow */
	/* Given an id and Escrow Capability, create a new Escrow and submit it to the PonsEscrowContract for consummation */
	/* This function returns an Escrow resource, which the caller must place in the location specified by the provided Escrow Capability, otherwise the Escrow cannot be consummated. */
	pub fun submitEscrow
	( id : String, escrowCap : Capability<&Escrow>
	, resources : @EscrowResource, requirement : EscrowResourceDescription, fulfillment : EscrowFulfillment
	) : @Escrow {
		var escrow <- create Escrow (id: id, resources: <- resources, requirement: requirement, fulfillment: fulfillment)
		let overwrittenEscrowCap = PonsEscrowContract .escrowCaps .insert (key: id, escrowCap)
		if overwrittenEscrowCap != nil {
			panic ("Another escrow already exists with the same ID") }
		emit PonsEscrowSubmitted (id: id, address: escrowCap .address, heldResourceDescription: escrow .heldResourceDescription, requirement: requirement)
		return <- escrow }

	/* This function allows the Pons account to consummate an Escrow using the specified method of consummation */
	access(account) fun consummateEscrow (id : String, consummation : ((@EscrowResource): @EscrowResource)) : Void {
		if ! PonsEscrowContract .escrowCaps .containsKey (id) {
			panic ("No active Escrow with the ID `" .concat (id) .concat ("` exists")) }
		let escrowRef = PonsEscrowContract .escrowCaps [id] !.borrow () !
		let fulfilledResourceDescription = escrowRef .consummate (consummation)
		emit PonsEscrowConsummated (
			id: escrowRef .id,
			address: escrowRef .owner !.address,
			heldResourceDescription: escrowRef .heldResourceDescription,
			requirement: escrowRef .requirement,
			fulfilledResourceDescription: fulfilledResourceDescription ) }

	/* API to terminate an Escrow */
	/* This function allows the any account holding a reference to an Escrow to terminate it */
	pub fun terminateEscrow (_ escrowRef : &Escrow) : Void {
		escrowRef .terminate ()
		emit PonsEscrowTerminated (id: escrowRef .id, address: escrowRef .owner !.address, heldResourceDescription: escrowRef .heldResourceDescription, requirement: escrowRef .requirement) }

	/* This function allows the Pons account to dismiss any unnecessary Escrows */
	access(account) fun dismissEscrow (id : String) : Void {
		if ! PonsEscrowContract .escrowCaps .containsKey (id) {
			panic ("No active Escrow with the ID `" .concat (id) .concat ("` exists")) }
		let escrowCap = PonsEscrowContract .escrowCaps .remove (key: id) !
		emit PonsEscrowDismissed (id: id, address: escrowCap .address) }



	/* Gets the EscrowResourceDescription corresponding to resources in an EscrowResource */
	pub fun resourceDescription (_ escrowResourceRef : &EscrowResource) : EscrowResourceDescription {
		let flowUnits = PonsUtils.FlowUnits (escrowResourceRef .flowVault .balance)
		let ponsNftIds : [String] = []

		var index = 0
		while index < escrowResourceRef .ponsNfts .length {
			ponsNftIds .append (escrowResourceRef .ponsNfts [index] .nftId)
			index = index + 1 }

		return EscrowResourceDescription (flowUnits: flowUnits, ponsNftIds: ponsNftIds) }

	/* Checks whether the provided EscrowResource satisfy the EscrowResourceDescription */
	pub fun satisfiesResourceDescription
	( _ escrowResourceRef : &EscrowResource
	, _ escrowResourceDescription : EscrowResourceDescription
	) : Bool {
		if ! PonsUtils.FlowUnits (escrowResourceRef .flowVault .balance) .isAtLeast (escrowResourceDescription .flowUnits) {
			return false }
		let ponsNftIds : [String] = []

		var index = 0
		while index < escrowResourceRef .ponsNfts .length {
			ponsNftIds .append (escrowResourceRef .ponsNfts [index] .nftId)
			index = index + 1 }
		for requiredNftId in escrowResourceDescription .getPonsNftIds () {
			if ! ponsNftIds .contains (requiredNftId) {
				return false } }
		return true }

	/* Transfer all the resources held in the provided EscrowResource using the EscrowFulfillment */
	pub fun fullfillResource (_ resources : @EscrowResource, _ fulfillment : EscrowFulfillment) : Void {
		var fulfillmentFlowVault <- resources .flowVault .withdraw (amount: resources .flowVault .balance)
		fulfillment .receivePaymentCap .borrow () !.deposit (from: <- fulfillmentFlowVault)

		while resources .ponsNfts .length > 0 {
			fulfillment .receiveNftCap .borrow () !.depositNft (<- resources .ponsNfts .remove (at: 0) !) }
		
		destroy resources }


	init () {
		self .escrowCaps = {}

		// Create one instance of EscrowManager and store it in the Pons account storage
		self .account .save (<- create EscrowManager (), to: /storage/escrowManager)

		// Emit the PonsEscrowContract initialisation event
		emit PonsEscrowContractInit () } }
