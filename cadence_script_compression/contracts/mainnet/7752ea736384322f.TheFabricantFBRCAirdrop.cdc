import FungibleToken from 0xf233dcee88fe0abe
import FBRC from 0xfc91de5e6566cc7c

// This contract allows an admin to allocate airdrop amounts to addresses
// allowing these addresses to receive FBRC by calling claimFBRC
pub contract TheFabricantFBRCAirdrop{

    pub event AddressClaimAdded(address: Address, claimName: String, claimAmount: UFix64)
    
    pub event AddressClaimed(address: Address, claimName: String, claimAmount: UFix64)

    pub event MinterChanged(address: Address)

    // dictionary of addresses and the amount they can currently claim
    access(self) var addressClaim: {Address: {String: ClaimStruct}}

    access(contract) var fbrcMinterCapability: Capability<&FBRC.Administrator>?

    pub let AdminStoragePath: StoragePath
    pub let ClaimerStoragePath: StoragePath

    pub struct ClaimStruct {

        pub let claimAmount: UFix64
        pub var hasClaimed: Bool

        init (
            claimAmount: UFix64
        ){
            self.claimAmount = claimAmount
            self.hasClaimed = false
        }

        pub fun setToHasClaimed() {
            pre {
                self.hasClaimed == false:
                "amount has been claimed"
            }
            self.hasClaimed = true
        }
    }
    
    pub resource Admin{

        //add claim to address map
        pub fun addClaim(address: Address, claimName: String, claimAmount: UFix64){
            let claimStruct = ClaimStruct(claimAmount: claimAmount)
            //case1: address not in mapping
            if(TheFabricantFBRCAirdrop.addressClaim[address] == nil){
                let map: {String: ClaimStruct} = {}
                map[claimName] = claimStruct
                TheFabricantFBRCAirdrop.addressClaim[address] = map
                emit AddressClaimAdded(address: address, claimName: claimName, claimAmount: claimAmount)
            //case2: address in mapping and claimName not in mapping
            } else if(TheFabricantFBRCAirdrop.addressClaim[address]![claimName] == nil) {
                let map = TheFabricantFBRCAirdrop.addressClaim[address]!
                map[claimName] = claimStruct
                TheFabricantFBRCAirdrop.addressClaim[address] = map  
                emit AddressClaimAdded(address: address, claimName: claimName, claimAmount: claimAmount)  
            //case3: address in mapping and claimName in mapping          
            } else {
                panic("claim is already in claimlist")
            }
        }

        // change contract royalty address
        pub fun setFBRCMinterCap(fbrcMinterCap: Capability<&FBRC.Administrator>) {
            pre {
                fbrcMinterCap.borrow() != nil: 
                    "Admin Minter Capability invalid"
            }
            TheFabricantFBRCAirdrop.fbrcMinterCapability = fbrcMinterCap
            emit MinterChanged(address: fbrcMinterCap.address)
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

    }

    pub resource Claimer {

        //users can claim FBRC if they have minted an item
        pub fun claimFBRC(claimName: String, fbrcCap: Capability<&FBRC.Vault{FungibleToken.Receiver}>) {     
            
            //Make sure the address has a claimable amount of FBRC
            pre {
                TheFabricantFBRCAirdrop.addressClaim[self.owner!.address] != nil:
                    "this address has no claimable FBRC"
                TheFabricantFBRCAirdrop.addressClaim[self.owner!.address]![claimName] != nil:
                    "this address has no claimable FBRC for this claim"            
                TheFabricantFBRCAirdrop.addressClaim[self.owner!.address]![claimName]!.hasClaimed == false:
                    "this address has already claimed this"
            }
            
            // get claimAddress
            let claimAddress = self.owner!.address

            // get claimAmount
            let claimAmount = TheFabricantFBRCAirdrop.addressClaim[claimAddress]![claimName]!.claimAmount

            let claimStruct = TheFabricantFBRCAirdrop.addressClaim[claimAddress]![claimName]!

            // set hasClaimed to be true
            claimStruct.setToHasClaimed()

            // reinsert to addressClaim map
            let map = TheFabricantFBRCAirdrop.addressClaim[claimAddress]!
            map[claimName] = claimStruct
            TheFabricantFBRCAirdrop.addressClaim[claimAddress] = map

            //mint fbrc from contract minter resource
            let fbrcAdmin = TheFabricantFBRCAirdrop.fbrcMinterCapability!.borrow()!
            
            let fbrcMinter <- fbrcAdmin.createNewMinter(allowedAmount: claimAmount)
        
            let mintedVault <- fbrcMinter.mintTokens(amount: claimAmount)

            let recipientFBRCVault = fbrcCap.borrow()??
                                        panic("FBRC Vault Capability invalid")

            //deposit fbrc to claimer's fbrc vault
            recipientFBRCVault.deposit(from: <- mintedVault)

            destroy fbrcMinter
            emit AddressClaimed(address: claimAddress, claimName: claimName, claimAmount: claimAmount)
        }

        // destructor
        //
        destroy () {}

        // initializer
        //
        init () {}
    }

    pub fun createNewClaimer(): @Claimer {
        return <-create Claimer()
    }

    // getter function for addressClaim
    pub fun getAddressClaim(): {Address: {String: ClaimStruct}} {
        return TheFabricantFBRCAirdrop.addressClaim
    }
    
    init() {
        self.fbrcMinterCapability = nil
        self.addressClaim = {}
        self.AdminStoragePath = /storage/FBRCAirdropAdmin0021
        self.ClaimerStoragePath = /storage/FBRCAirdropClaimer0021
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}