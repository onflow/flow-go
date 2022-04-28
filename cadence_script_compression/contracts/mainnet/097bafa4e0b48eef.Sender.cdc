pub contract Sender {

	pub let storagePath: StoragePath
	
	/// This is just an empty resource we create in storage, you can safely send a reference to it to obtain msg.sender
	pub resource Token { }

	pub fun create() : @Sender.Token {
		return <- create Token()
	}

	init() {
		self.storagePath = /storage/findSender
	}

}

