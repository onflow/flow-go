transaction(addresses: [Address], path: StoragePath) {
	prepare(signer: AuthAccount) {
		signer.load<[Address]>(from: path)
		signer.save(addresses, to: path)
	}
}
