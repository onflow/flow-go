import MotoGPPack from 0xa49cc0ee46c54bfb

pub contract MotoGPAdmin{

    pub fun getVersion(): String {
        return "0.7.8"
    }
    
    // Admin
    // the admin resource is defined so that only the admin account
    // can have this resource. 
    
    pub resource Admin {

        // addPackType
        // adds a new packType to the packTypes dictionary
        // at the beginning of the MotoGPPack contract.
        //
        // The number of cards represents the amount of Cards
        // that will be minted when a Pack of this packType
        // is opened.
        //
        pub fun addPackType(packType: UInt64, numberOfCards: UInt64) {
            pre {
                MotoGPPack.packTypes[packType] == nil:
                    "This pack type already exists!"
            }
            // Adds this pack type
            MotoGPPack.packTypes[packType] = MotoGPPack.PackType(_packType: packType, _numberOfCards: numberOfCards)
        }

        // mintPacks
        // a function that mints new Pack(s) and deposits
        // them into the admin's Pack Collection.
        //
        pub fun mintPacks(packType: UInt64, numberOfPacks: UInt64, packNumbers: [UInt64]) {
            pre {
                MotoGPPack.packTypes[packType] != nil:
                    "This pack type does not exist!"
                numberOfPacks > (0 as UInt64):
                    "Number of packs to be minted must be greater than 0"
                Int(numberOfPacks) == packNumbers.length:
                    "The amount of packNumbers passed in is not equal to the numberOfPacks to be minted"
            }
            
            // Gets the admin's Pack Collection
            let adminPackCollection = MotoGPAdmin.account.borrow<&MotoGPPack.Collection{MotoGPPack.IPackCollectionPublic}>(from: /storage/motogpPackCollection)!

            var i: UInt64 = 0
            while i < numberOfPacks {
                let newPack <- MotoGPPack.createPack(packNumber: packNumbers[i], packType: packType)
            
                // Adds the new pack to the admin's Pack Collection
                adminPackCollection.deposit(token: <- newPack)

                i = i + (1 as UInt64)
            }
        }

        // createAdmin
        // only an admin can ever create
        // a new Admin resource
        //
        pub fun createAdmin(): @Admin {
            return <- create Admin()
        }

        init() {
            
        }
    }

    init() {
        self.account.save(<- create Admin(), to: /storage/motogpAdmin)
    }
}