pub contract AAReferralManager {
    access(self) let refs: { Address: Address}

    pub event Referral(owner: Address, referrer: Address)

    pub let AdminStoragePath: StoragePath

    pub resource Administrator {
      pub fun setRef(owner: Address, referrer: Address) {
        AAReferralManager.refs[owner] = referrer

        emit  Referral(owner: owner, referrer: referrer)
      } 
    }

    pub fun referrerOf(owner: Address): Address? {
      return self.refs[owner]
    }

    init() {
        self.refs = {}

        self.AdminStoragePath = /storage/AAReferralManagerAdmin
        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }

}