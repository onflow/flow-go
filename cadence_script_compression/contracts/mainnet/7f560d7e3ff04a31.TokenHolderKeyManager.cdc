import KeyManager from 0x840b99b76051d886

	// The TokenHolderKeyManager contract is an implementation of
	// the KeyManager interface intended for use by FLOW token holders.
	//
	// One instance is deployed to each token holder account.
	// Deployment is executed a with signature from the administrator,
	// allowing them to take possession of a KeyAdder resource
	// upon initialization.
	pub contract TokenHolderKeyManager: KeyManager {

	access(contract) fun addPublicKey(_ publicKey: [UInt8]) {
		self.account.addPublicKey(publicKey)
	}
	
	pub resource KeyAdder: KeyManager.KeyAdder {

		pub let address: Address

		pub fun addPublicKey(_ publicKey: [UInt8]) {
		TokenHolderKeyManager.addPublicKey(publicKey)
		}

		init(address: Address) {
		self.address = address
		}
	}

	init(admin: AuthAccount, path: StoragePath) {
		let keyAdder <- create KeyAdder(address: self.account.address)

		admin.save(<- keyAdder, to: path)
	}
	}
	