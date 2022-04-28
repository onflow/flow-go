pub contract RelatedAccounts {

	pub let storagePath: StoragePath
	pub let publicPath: PublicPath

	pub event RelatedFlowAccountAdded(name: String, address: Address, related: Address)
	pub event RelatedFlowAccountRemoved(name: String, address: Address, related: Address)

	pub struct AccountInformation{
		pub let name:String
		pub let address:Address?
		pub let network:String //do not use enum because of contract upgrade
		pub let otherAddress: String? //other networks besides flow may be not support Address

		init(name:String, address:Address?, network:String, otherAddress:String?){
			self.name=name
			self.address=address
			self.network=network
			self.otherAddress=otherAddress
		}
	}

	pub resource interface Public{
		pub fun getFlowAccounts() : {String: Address} 
	}
	/// This is just an empty resource we create in storage, you can safely send a reference to it to obtain msg.sender
	pub resource Accounts: Public {


		access(self) let accounts: { String: AccountInformation}

		pub fun getFlowAccounts() : {String: Address} {
			let items : {String: Address} ={}
			for account in self.accounts.keys {
				let item = self.accounts[account]!
				if item.network == "Flow" {
					items[item.name]=item.address!
				}
			}
			return items
		}

		pub fun setFlowAccount(name: String, address:Address) {
			self.accounts[name] = AccountInformation(name: name, address:address, network: "Flow", otherAddress:"")
			emit RelatedFlowAccountAdded(name:name, address: self.owner!.address, related:address)
		}

		pub fun deleteAccount(name: String) {
			let item =self.accounts.remove(key: name)!
			if item.network == "Flow" {
				emit RelatedFlowAccountRemoved(name:name,address: self.owner!.address, related: item.address!)
			}
		}

		init() {
			self.accounts={}
		}
	}

	pub fun createEmptyAccounts() : @Accounts{
		return <- create Accounts()
	}

	pub fun findRelatedFlowAccounts(address:Address) : { String: Address} {
		let cap = getAccount(address).getCapability<&Accounts{Public}>(self.publicPath)
		if !cap.check(){
			return {}
		}

		return cap.borrow()!.getFlowAccounts()
	}

	init() {

		self.storagePath = /storage/findAccounts
		self.publicPath = /public/findAccounts
	}

}

