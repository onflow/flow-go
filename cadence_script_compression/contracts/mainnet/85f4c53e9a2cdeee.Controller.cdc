import FungibleToken from 0xf233dcee88fe0abe
pub contract Controller {

    // Variable size dictionary of TokenStructure
    access(contract) var allSocialTokens: {String: TokenStructure}

    // Events
    // Emitted when a Reserve is incremented while minting social tokens
    pub event incrementReserve(_ newReserve:UFix64)
    // Emitted when a Reserve is decremented while burning social tokens
    pub event decrementReserve(_ newReserve:UFix64)
    // Emitted when a IssuedSupply is incremented after minting social tokens
    pub event incrementIssuedSupply(_ amount: UFix64)
    // Emitted when a IssuedSupply is decremented after minting social tokens
    pub event decrementIssuedSupply(_ amount: UFix64)
    // Emitted when a social Token is registered 
    pub event registerToken(_ tokenId: String, _ symbol: String, _ maxSupply: UFix64, _ artist: Address)
    // Emitted when a Percentage of social token is updated 
    pub event updatePercentage(_ percentage: UFix64)
    // Emitted when fee for social token is updated 
    pub event updateFeeSplitterDetail(_ tokenId: String)

    // Paths
    pub let AdminStoragePath: StoragePath
    pub let SocialTokenResourceStoragePath: StoragePath

    pub let SpecialCapabilityPrivatePath: PrivatePath
    pub let SocialTokenResourcePrivatePath: PrivatePath

    // A structure that contains all the data related to the Token 
    pub struct TokenStructure {
        pub var tokenId: String
        pub var symbol: String
        pub var issuedSupply: UFix64
        pub var maxSupply: UFix64
        pub var artist: Address
        pub var slope: UFix64
        access(contract) var feeSplitterDetail: {Address: FeeStructure}
        pub var reserve: UFix64
        pub var tokenResourceStoragePath: StoragePath
        pub var tokenResourcePublicPath: PublicPath
        pub var socialMinterStoragePath: StoragePath
        pub var socialMinterPublicPath: PublicPath
        pub var socialBurnerStoragePath: StoragePath
        pub var socialBurnerPublicPath: PublicPath

        init(_ tokenId: String, _ symbol: String, _ maxSupply: UFix64, _ artist: Address,
            _ tokenStoragePath: StoragePath, _ tokenPublicPath: PublicPath,
            _ socialMinterStoragePath: StoragePath, _ socialMinterPublicPath: PublicPath,
            _ socialBurnerStoragePath: StoragePath, _ socialBurnerPublicPath: PublicPath) {
            self.tokenId = tokenId
            self.symbol = symbol
            self.issuedSupply = 0.0
            self.maxSupply = maxSupply
            self.artist = artist
            self.slope = 0.5
            self.feeSplitterDetail = {}
            self.reserve = 0.0
            self.tokenResourceStoragePath = tokenStoragePath
            self.tokenResourcePublicPath = tokenPublicPath
            self.socialMinterStoragePath = socialMinterStoragePath
            self.socialMinterPublicPath = socialMinterPublicPath
            self.socialBurnerStoragePath = socialBurnerStoragePath
            self.socialBurnerPublicPath = socialBurnerPublicPath
        }

        pub fun incrementReserve(_ newReserve: UFix64) {
            pre {
                newReserve != nil: "reserve must not be null"
                newReserve > 0.0 : "reserve must be greater than zero"
            }
            self.reserve = self.reserve + newReserve
            emit incrementReserve(newReserve)
        }

        pub fun decrementReserve(_ newReserve: UFix64) {
            pre {
                newReserve != nil: "reserve must not be null"
                newReserve > 0.0 : "reserve must be greater than zero"
            }
            self.reserve = self.reserve - newReserve
            emit decrementReserve(newReserve)
        }

        pub fun incrementIssuedSupply(_ amount: UFix64) {
            pre{
                self.issuedSupply + amount <= self.maxSupply: "max supply reached"
            }
            self.issuedSupply = self.issuedSupply + amount
            emit incrementIssuedSupply(amount)
        }

        pub fun decrementIssuedSupply(_ amount: UFix64) {
            pre {
                self.issuedSupply - amount >= 0.0 : "issued supply must not be zero"
            }
            self.issuedSupply = self.issuedSupply - amount
            emit decrementIssuedSupply(amount)
        
        }

        pub fun setFeeSplitterDetail(_ feeSplitterDetail: {Address: FeeStructure}) {
            pre {
            }
            self.feeSplitterDetail = feeSplitterDetail
        }

        pub fun getFeeSplitterDetail(): {Address: FeeStructure}{
            return self.feeSplitterDetail           
        }
    }

    pub resource interface SpecialCapability {
        pub fun registerToken( _ symbol: String, _ maxSupply: UFix64, _ artist: Address,
            _ tokenStoragePath: StoragePath, _ tokenPublicPath: PublicPath,
            _ socialMinterStoragePath: StoragePath, _ socialMinterPublicPath: PublicPath,
            _ socialBurnerStoragePath: StoragePath, _ socialBurnerPublicPath: PublicPath)
        pub fun updateFeeSplitterDetail( _ tokenId: String, _ feeSplitterDetail: {Address: FeeStructure})
    }

    pub resource interface UserSpecialCapability {
        pub fun addCapability(cap: Capability<&{SpecialCapability}>)
    }

    pub resource interface SocialTokenResourcePublic {
        pub fun incrementIssuedSupply(_ tokenId: String, _ amount: UFix64)
        pub fun decrementIssuedSupply(_ tokenId: String, _ amount: UFix64)
        pub fun incrementReserve(_ tokenId: String, _ newReserve: UFix64)
        pub fun decrementReserve(_ tokenId: String, _ newReserve: UFix64)
    }

    pub resource Admin: SpecialCapability {
        pub fun registerToken( _ symbol: String, _ maxSupply: UFix64, _ artist: Address,
            _ tokenStoragePath: StoragePath, _ tokenPublicPath: PublicPath,
            _ socialMinterStoragePath: StoragePath, _ socialMinterPublicPath: PublicPath,
            _ socialBurnerStoragePath: StoragePath, _ socialBurnerPublicPath: PublicPath) {
            pre {
                symbol !=nil: "symbol must not be null"
                maxSupply > 0.0: "max supply must be greater than zero"
            }
            let artistAddress = artist
            let resourceOwner = self.owner!.address
            let tokenId = (symbol.concat("_")).concat(artistAddress.toString()) 
            assert(Controller.allSocialTokens[tokenId] == nil, message:"token already registered")
            Controller.allSocialTokens[tokenId]= Controller.TokenStructure(
                tokenId, symbol, maxSupply, artistAddress,
                tokenStoragePath, tokenPublicPath,
                socialMinterStoragePath, socialMinterPublicPath,
                socialBurnerStoragePath, socialBurnerPublicPath) 
            emit registerToken(tokenId, symbol, maxSupply, artistAddress)
        } 

        pub fun updateFeeSplitterDetail( _ tokenId: String, _ feeSplitterDetail: {Address: FeeStructure}) {
            Controller.allSocialTokens[tokenId]!.setFeeSplitterDetail(feeSplitterDetail)
            emit updateFeeSplitterDetail(tokenId)
        }
    }

    pub resource SocialTokenResource : SocialTokenResourcePublic , UserSpecialCapability {
        // a variable that store user capability to utilize methods 
        access(contract) var capability: Capability<&{SpecialCapability}>?
        // method which provide capability to user to utilize methods
        pub fun addCapability (cap: Capability<&{SpecialCapability}>){
            pre {
                // we make sure the SpecialCapability is 
                // valid before executing the method
                cap.borrow() != nil: "could not borrow a reference to the SpecialCapability"
                self.capability == nil: "resource already has the SpecialCapability"
            }
            // add the SpecialCapability
            self.capability = cap
        }

        //method to increment issued supply, only access by the verified user
        pub fun incrementIssuedSupply(_ tokenId: String, _ amount: UFix64){
            pre {
                amount > 0.0: "Amount must be greator than zero"
                tokenId != "" : "token id must not be null"
                Controller.allSocialTokens[tokenId]!=nil : "token id must not be null"
            }
            Controller.allSocialTokens[tokenId]!.incrementIssuedSupply(amount)
        }
        
        // method to decrement issued supply, only access by the verified user
        pub fun decrementIssuedSupply(_ tokenId: String, _ amount: UFix64) {
            pre {
                amount > 0.0: "Amount must be greator than zero"
                tokenId != "" : "token id must not be null"
                Controller.allSocialTokens[tokenId]!=nil : "token id must not be null"
            }
            Controller.allSocialTokens[tokenId]!.decrementIssuedSupply(amount)
        }

        // method to increment reserve of a token, only access by the verified user
        pub fun incrementReserve(_ tokenId: String, _ newReserve: UFix64) {
            pre {
                newReserve != nil: "reserve must not be null"
                newReserve > 0.0 : "reserve must be greater than zero"
                
            }
            Controller.allSocialTokens[tokenId]!.incrementReserve(newReserve)
        }

        // method to decrement reserve of a token, only access by the verified user
        pub fun decrementReserve(_ tokenId: String, _ newReserve: UFix64) {
        
            pre {
                newReserve != nil: "reserve must not be null"
                newReserve > 0.0: "reserve must be greater than zero"
                
            }
            Controller.allSocialTokens[tokenId]!.decrementReserve(newReserve)
        }
        
        init(){
            self.capability = nil
        
        }
    }

    // A structure that contains all the data related to the Fee
    pub struct FeeStructure {
        pub var percentage: UFix64

        init(_ percentage: UFix64) {
            self.percentage = percentage
        }
        // method to update the percentage of the token
        access(account) fun updatePercentage(_ percentage: UFix64) { 
            pre {
                percentage > 0.0: "Percentage should be greater than zero"
            }
            self.percentage = percentage
            emit updatePercentage(percentage)
        }
    }

    // method to get all the token details
    pub fun getTokenDetails(_ tokenId: String): Controller.TokenStructure {
        pre {
            tokenId != nil: "token id must not be null"
            Controller.allSocialTokens[tokenId]!.tokenId == tokenId: "token id is not same"
            }
        return self.allSocialTokens[tokenId]! 
    }
    
    // method to create a SocialTokenResource
    pub fun createSocialTokenResource(): @SocialTokenResource {
        return <- create SocialTokenResource()
    }

    init() {
        self.allSocialTokens= {}
        self.AdminStoragePath = /storage/ControllerAdmin
        self.SocialTokenResourceStoragePath = /storage/ControllerSocialTokenResource

        self.SpecialCapabilityPrivatePath = /private/ControllerSpecialCapability
        self.SocialTokenResourcePrivatePath = /private/ControllerSocialTokenResourcePrivate

        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
        self.account.link<&{SpecialCapability}>(self.SpecialCapabilityPrivatePath, target: self.AdminStoragePath)
    }
}
