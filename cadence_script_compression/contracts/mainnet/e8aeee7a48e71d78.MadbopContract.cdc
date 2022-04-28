import MadbopNFTs from 0xe8aeee7a48e71d78
import NonFungibleToken from 0x1d7e57aa55817448

pub contract MadbopContract {

    // event for madbop data initalization
    pub event MadbopDataInitialized(brandId: UInt64, jukeboxSchema: [UInt64], nftSchema :[UInt64])
    // event when madbop data is updated
    pub event MadbopDataUpdated(brandId: UInt64, jukeboxSchema: [UInt64], nftSchema: [UInt64])
    // event when a jukebox is created
    pub event JukeboxCreated(templateId: UInt64, openDate: UFix64)
    // event when a jukebox is opened
    pub event JukeboxOpened(nftId: UInt64, receiptAddress: Address?)
    // path for jukebox storage
    pub let JukeboxStoragePath : StoragePath
    // path for jukebox public
    pub let JukeboxPublicPath : PublicPath
    // dictionary to store Jukebox data
    access(self) var allJukeboxes: {UInt64: JukeboxData}
    // dictionary to store madbop data
    access(self) var madbopData: MadbopData
    // capability of MadbopNFTs of NFTMethods to call the mint function on this capability
    access(contract) let adminRef : Capability<&{MadbopNFTs.NFTMethodsCapability}>

    // all methods are accessed by only the admin
    pub struct MadbopData {
        pub var brandId: UInt64
        access(contract) var jukeboxSchema: [UInt64]
        access(contract) var nftSchema: [UInt64]

        init(brandId: UInt64, jukeboxSchema: [UInt64], nftSchema :[UInt64]) {
            self.brandId = brandId
            self.jukeboxSchema = jukeboxSchema
            self.nftSchema = nftSchema
        }

        pub fun updateData(brandId: UInt64, jukeboxSchema: [UInt64], nftSchema: [UInt64]) {
            self.brandId = brandId
            self.jukeboxSchema = jukeboxSchema
            self.nftSchema = nftSchema
        }
    }

    pub struct JukeboxData {
        pub let templateId: UInt64
        pub let openDate: UFix64

        init(templateId: UInt64, openDate: UFix64) {
            self.templateId = templateId
            self.openDate = openDate
        }
    }

    pub resource interface JukeboxPublic {
        // making this function public to call by other users
        pub fun openJukebox(jukeboxNFT: @NonFungibleToken.NFT, receiptAddress: Address)
    }

    pub resource Jukebox: JukeboxPublic {
        pub fun createJukebox(templateId: UInt64, openDate: UFix64) {
            pre {
                templateId != nil: "template id must not be null"
                MadbopContract.allJukeboxes[templateId] == nil: "Jukebox already created with the given template id"
                openDate > 0.0 : "Open date should be greater than zero"
            }    
            let templateData = MadbopNFTs.getTemplateById(templateId: templateId)
            assert(templateData != nil, message: "data for given template id does not exist")
            // brand Id of template must be Madbop brand Id
            assert(templateData.brandId == MadbopContract.madbopData.brandId, message: "Invalid Brand id")
            // template must be the Jukebox template
            assert(MadbopContract.madbopData.jukeboxSchema.contains(templateData.schemaId), message: "Template does not contain Jukebox standard")
            assert(openDate >= getCurrentBlock().timestamp, message: "open date must be greater than current date")
            // check all templates under the jukexbox are created or not
            var allNftTemplateExists = true;
            let templateImmutableData = templateData.getImmutableData()
            let allIds = templateImmutableData["nftTemplates"]! as! [AnyStruct]
            assert(allIds.length <= 5, message: "templates limit exceeded")
            for tempID in allIds {
                var castedTempId = UInt64(tempID as! Int)
                let nftTemplateData = MadbopNFTs.getTemplateById(templateId: castedTempId)
                if(nftTemplateData == nil) {
                    allNftTemplateExists = false
                    break
                }
            }
            assert(allNftTemplateExists, message: "Invalid NFTs")
            let newJukebox = JukeboxData(templateId: templateId, openDate: openDate)
            MadbopContract.allJukeboxes[templateId] = newJukebox
            emit JukeboxCreated(templateId: templateId, openDate: openDate)
        }

        // update madbop data function will be updated when a new user creates a new brand with its own data
        // and pass new user details
        pub fun updateMadbopData(brandId: UInt64, jukeboxSchema: [UInt64], nftSchema: [UInt64]) {
            pre {
                brandId != nil: "brand id must not be null"
                jukeboxSchema != nil: "jukebox schema array must not be null"
                nftSchema != nil: "nft schema array must not be null"
            }
            MadbopContract.madbopData.updateData(brandId: brandId, jukeboxSchema: jukeboxSchema, nftSchema: nftSchema)
            emit MadbopDataUpdated(brandId: brandId, jukeboxSchema: jukeboxSchema, nftSchema: nftSchema)
        }

        // open jukebox function called by user to open specific jukebox to mint all the nfts in and transfer it to
        // the user address
        pub fun openJukebox(jukeboxNFT: @NonFungibleToken.NFT, receiptAddress: Address) {
            pre {
                jukeboxNFT != nil : "jukebox nft must not be null"
                receiptAddress != nil : "receipt address must not be null"
            }
            var jukeboxMadbopNFTData = MadbopNFTs.getMadbopNFTDataById(nftId: jukeboxNFT.id)
            var jukeboxTemplateData = MadbopNFTs.getTemplateById(templateId: jukeboxMadbopNFTData.templateID)
            // check if it is regiesterd or not
            assert(MadbopContract.allJukeboxes[jukeboxMadbopNFTData.templateID] != nil, message: "Jukebox is not registered") 
            // check if current date is greater or equal than opendate 
            assert(MadbopContract.allJukeboxes[jukeboxMadbopNFTData.templateID]!.openDate <= getCurrentBlock().timestamp, message: "current date must be greater than or equal to the open date")
            let templateImmutableData = jukeboxTemplateData.getImmutableData()
            let allIds = templateImmutableData["nftTemplates"]! as! [AnyStruct]
            assert(allIds.length <= 5, message: "templates limit exceeded")
            for tempID in allIds {
                var castedTempId = UInt64(tempID as! Int)
                MadbopContract.adminRef.borrow()!.mintNFT(templateId: castedTempId, account: receiptAddress)
            }
            emit JukeboxOpened(nftId: jukeboxNFT.id, receiptAddress: self.owner?.address)
            destroy jukeboxNFT
        }
    }

    pub fun getAllJukeboxes(): {UInt64: JukeboxData} {
        pre {
            MadbopContract.allJukeboxes != nil: "jukebox does not exist"
        }
        return MadbopContract.allJukeboxes
    }

    pub fun getJukeboxById(jukeboxId: UInt64): JukeboxData {
        pre {
            MadbopContract.allJukeboxes[jukeboxId] != nil: "jukebox id does not exist"
        }
        return MadbopContract.allJukeboxes[jukeboxId]!
    }

    pub fun getMadbopData(): MadbopData {
        pre {
            MadbopContract.madbopData != nil: "data does not exist"
        }
        return MadbopContract.madbopData
    }

    init() {
        self.allJukeboxes = {}
        self.madbopData = MadbopData(brandId: 0, jukeboxSchema: [], nftSchema: [])
        var adminPrivateCap = self.account.getCapability
            <&{MadbopNFTs.NFTMethodsCapability}>(MadbopNFTs.NFTMethodsCapabilityPrivatePath)
        self.adminRef = adminPrivateCap
        self.JukeboxStoragePath = /storage/MadbopJukebox
        self.JukeboxPublicPath = /public/MadbopJukebox
        self.account.save(<- create Jukebox(), to: self.JukeboxStoragePath)
        self.account.link<&{JukeboxPublic}>(self.JukeboxPublicPath, target: self.JukeboxStoragePath)
        emit MadbopDataInitialized(brandId: 0, jukeboxSchema: [], nftSchema: [])
    }
}