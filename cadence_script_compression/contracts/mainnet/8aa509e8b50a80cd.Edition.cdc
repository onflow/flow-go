import FungibleToken from 0xf233dcee88fe0abe

// Common information for all copies of the same item
pub contract Edition {

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // The total amount of editions that have been created
    pub var totalEditions: UInt64

    // Struct to display and handle commissions
    pub struct CommissionStructure {
        pub let firstSalePercent: UFix64
        pub let secondSalePercent: UFix64
        pub let description: String
     
        init(          
            firstSalePercent: UFix64, 
            secondSalePercent: UFix64,
            description: String    
        ) {           
            self.firstSalePercent = firstSalePercent
            self.secondSalePercent = secondSalePercent
            self.description = description
        }
    }

    // Events  
    pub event CreateEdition(editionId: UInt64, maxEdition: UInt64) 
    pub event ChangeCommision(editionId: UInt64) 
    pub event ChangeMaxEdition(editionId: UInt64, maxEdition: UInt64) 

    // Edition's status(commission and amount copies of the same item)
    pub struct EditionStatus {
        pub let royalty: { Address: CommissionStructure }  
        pub let editionId: UInt64  
        pub let maxEdition: UInt64      

        init(
            royalty: { Address: CommissionStructure },
            editionId: UInt64,
            maxEdition: UInt64      
        ) {
            self.royalty = royalty                    
            self.editionId = editionId
            // Amount copies of the same item
            self.maxEdition = maxEdition
        }
    }

    // Attributes one edition, where stores royalty and amount of copies
    pub resource EditionItem {
        pub let editionId: UInt64
        pub var royalty: { Address: CommissionStructure }
        // Amount copies of the same item
        priv var maxEdition: UInt64  

        init(
            royalty: { Address: CommissionStructure },
            maxEdition: UInt64
        ) {
            Edition.totalEditions = Edition.totalEditions + (1 as UInt64)
            self.royalty = royalty                    
            self.editionId = Edition.totalEditions 
            self.maxEdition = maxEdition
        }

        // Get status of edition        
        pub fun getEdition(): EditionStatus {
            return EditionStatus(
                royalty: self.royalty,                      
                editionId: self.editionId,
                maxEdition: self.maxEdition
            )
        }

        // Change commision
        pub fun changeCommission(      
           royalty: { Address: CommissionStructure }     
        ) {
            self.royalty = royalty
            emit ChangeCommision(editionId: self.editionId)
        }

        // Change count of copies. This is used for Open Edition, because the eventual amount of copies are known only after finish of sale       
        pub fun changeMaxEdition (      
           maxEdition: UInt64     
        ) {
            pre {
               // Possible change this number only once after Open Edition would be completed
               self.maxEdition < UInt64(1) : "Forbid change max edition" 
            }

            self.maxEdition = maxEdition      

            emit ChangeMaxEdition(editionId: self.editionId, maxEdition: maxEdition) 
        }
       
        destroy() {
            log("destroy edition item")            
        }
    }    

    // EditionCollectionPublic is a resource interface that restricts users to
    // retreiving the edition's information
    pub resource interface EditionCollectionPublic {
        pub fun getEdition(_ id: UInt64): EditionStatus?
    }

    //EditionCollection contains a dictionary EditionItems and provides
    // methods for manipulating EditionItems
    pub resource EditionCollection: EditionCollectionPublic  {

        // Edition Items
        access(account) var editionItems: @{UInt64: EditionItem} 

        init() {    
            self.editionItems <- {}
        }

        // Validate royalty
        priv fun validateRoyalty(       
            royalty: { Address: CommissionStructure }   
        ) {
            var firstSummaryPercent = 0.00
            var secondSummaryPercent = 0.00          

            for key in royalty.keys {

                firstSummaryPercent = firstSummaryPercent + royalty[key]!.firstSalePercent

                secondSummaryPercent = secondSummaryPercent + royalty[key]!.secondSalePercent

                let account = getAccount(key)

                let vaultCap = account.getCapability<&{FungibleToken.Receiver}>(/public/fusdReceiver)  

                if !vaultCap.check() { 
                    let panicMessage = "Account ".concat(key.toString()).concat(" does not provide fusd vault capability")
                    panic(panicMessage) 
                }
            }      

            if firstSummaryPercent != 100.00 { 
                panic("The first summary sale percent should be 100 %")
            }

            if secondSummaryPercent >= 100.00 { 
                panic("The second summary sale percent should be less than 100 %")
            }            
        }

        // Create edition (common information for all copies of the same item)
        pub fun createEdition(
            royalty: { Address: CommissionStructure },
            maxEdition: UInt64        
        ): UInt64 {

            self.validateRoyalty(royalty: royalty)            
           
            let item <- create EditionItem(
                royalty: royalty,
                maxEdition: maxEdition                  
            )

            let id = item.editionId

            // update the auction items dictionary with the new resources
            let oldItem <- self.editionItems[id] <- item
            
            destroy oldItem

            emit CreateEdition(editionId: id, maxEdition: maxEdition) 

            return id
        }
     
        pub fun getEdition(_ id: UInt64): EditionStatus? {
            if self.editionItems[id] == nil { 
                return nil
            }         

            // Get the edition item resources
            let itemRef = &self.editionItems[id] as &EditionItem
            return itemRef.getEdition()
        }

        //Change commission
        pub fun changeCommission(
            id: UInt64,
            royalty: { Address: CommissionStructure }   
        ) {
            pre {
                self.editionItems[id] != nil: "Edition does not exist"
            }
          
            self.validateRoyalty(royalty: royalty) 
            
            let itemRef = &self.editionItems[id] as &EditionItem
            
            itemRef.changeCommission(
                royalty: royalty            
            )
        }

        // Change count of copies. This is used for Open Edition, because the eventual amount of copies are unknown 
        pub fun changeMaxEdition(
            id: UInt64,
            maxEdition: UInt64
        ) {
            pre {
                self.editionItems[id] != nil: "Edition does not exist"
            }
            
            let itemRef = &self.editionItems[id] as &EditionItem
            itemRef.changeMaxEdition(
                maxEdition: maxEdition        
            )
        }
    
        destroy() {
            log("destroy edition collection")
            // destroy the empty resources
            destroy self.editionItems
        }
    }   

    // createEditionCollection returns a new createEditionCollection resource to the caller
    priv fun createEditionCollection(): @EditionCollection {
        let editionCollection <- create EditionCollection()

        return <- editionCollection
    }

    init() {
        self.totalEditions = (0 as UInt64)
        self.CollectionPublicPath = /public/NFTxtinglesEdition
        self.CollectionStoragePath = /storage/NFTxtinglesEdition

        let edition <- Edition.createEditionCollection()
        self.account.save(<- edition, to: Edition.CollectionStoragePath)         
        self.account.link<&{Edition.EditionCollectionPublic}>(Edition.CollectionPublicPath, target: Edition.CollectionStoragePath)
    }   
}