import SchmoesNFT from 0x6c4fe48768523577

pub contract SchmoesNFTAdmin {
    pub let AdminStoragePath: StoragePath

    pub resource Admin {
        pub fun setIsSaleActive(_ newIsSaleActive: Bool) {
            SchmoesNFT.setIsSaleActive(newIsSaleActive)
        }

        pub fun setPrice(_ newPrice: UFix64) {
            SchmoesNFT.setPrice(newPrice)
        }

        pub fun setMaxMintAmount(_ newMaxMintAmount: UInt64) {
            SchmoesNFT.setMaxMintAmount(newMaxMintAmount)
        }

        pub fun setIpfsBaseCID(_ ipfsBaseCID: String) {
            SchmoesNFT.setIpfsBaseCID(ipfsBaseCID)
        }

        pub fun setProvenance(_ provenance: String) {
            SchmoesNFT.setProvenance(provenance)
        }

        pub fun setProvenanceForEdition(_ edition: UInt64, _ provenance: String) {
            SchmoesNFT.setProvenanceForEdition(edition, provenance)
        }
        
        pub fun setSchmoeAsset(_ assetType: SchmoesNFT.SchmoeTrait, _ assetName: String, _ content: String) {
            SchmoesNFT.setSchmoeAsset(assetType, assetName, content)
        }

        pub fun batchUpdateSchmoeData(_ schmoeDataMap: { UInt64 : SchmoesNFT.SchmoeData }) {
            SchmoesNFT.batchUpdateSchmoeData(schmoeDataMap)
        }

        pub fun setEarlyLaunchTime(_ earlyLaunchTime: UFix64) {
            SchmoesNFT.setEarlyLaunchTime(earlyLaunchTime)
        }
        
        pub fun setLaunchTime(_ launchTime: UFix64) {
            SchmoesNFT.setLaunchTime(launchTime)
        }
        
        pub fun setIdsPerIncrement(_ idsPerIncrement: UInt64) {
            SchmoesNFT.setIdsPerIncrement(idsPerIncrement)
        }
        
         pub fun setTimePerIncrement (_ timePerIncrement: UInt64) {
            SchmoesNFT.setTimePerIncrement(timePerIncrement)
        }
    }

    pub init() {
        self.AdminStoragePath = /storage/schmoesNFTAdmin
        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}
