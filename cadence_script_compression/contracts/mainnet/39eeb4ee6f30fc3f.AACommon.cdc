pub contract AACommon {
    pub struct PaymentCut {
        // typicaly they are Storage, Insurance, Contractor
        pub let type: String
        pub let recipient: Address
        pub let rate: UFix64

        init (type: String, recipient: Address, rate: UFix64) {
          assert(rate >= 0.0 && rate <= 1.0, message: "Rate should be other than 0")
          
          self.type = type
          self.recipient = recipient
          self.rate = rate
        }
    }

    pub struct Payment {
        // typicaly they are Storage, Insurance, Contractor
        pub let type: String
        pub let recipient: Address
        pub let rate: UFix64
        pub let amount: UFix64

        init (type: String, recipient: Address, rate: UFix64, amount: UFix64) {
          self.type = type
          self.recipient = recipient
          self.rate = rate
          self.amount = amount
        }
    }

    pub fun itemIdentifier(type: Type, id: UInt64): String {
        return type.identifier.concat("-").concat(id.toString())
    }
}