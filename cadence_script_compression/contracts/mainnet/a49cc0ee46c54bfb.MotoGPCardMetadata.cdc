import MotoGPAdmin from 0xa49cc0ee46c54bfb
import ContractVersion from 0xa49cc0ee46c54bfb

// Contract to hold Metadata for MotoGPCards. Metadata is accessed using the Card's cardID (not the Card's id)
//
pub contract MotoGPCardMetadata: ContractVersion {

   pub fun getVersion():String {
       return "0.7.8"
   }

   pub struct Metadata {
       pub let cardID: UInt64
       pub let name: String
       pub let description: String
       pub let imageUrl: String
       // data contains all 'other' metadata fields, e.g. videoUrl, team, etc
       //
       pub let data: {String: String} 
       init(_cardID:UInt64, _name:String, _description:String, _imageUrl:String, _data:{String:String}){
           pre {
                !_data.containsKey("name") : "data dictionary contains 'name' key"
                !_data.containsKey("description") : "data dictionary contains 'description' key"
                !_data.containsKey("imageUrl") : "data dictionary contains 'imageUrl' key"
           }
           self.cardID = _cardID
           self.name = _name
           self.description = _description
           self.imageUrl = _imageUrl
           self.data = _data
       }
   }

   //Dictionary to hold all metadata with cardID as key
   //
   access(self) let metadatas: {UInt64: MotoGPCardMetadata.Metadata} 

   // Get all metadatas
   //
   pub fun getMetadatas(): {UInt64: MotoGPCardMetadata.Metadata} {
       return self.metadatas;
   }

   pub fun getMetadatasCount(): UInt64 {
       return UInt64(self.metadatas.length)
   }

   //Get metadata for a specific cardID
   //
   pub fun getMetadataForCardID(cardID: UInt64): MotoGPCardMetadata.Metadata? {
       return self.metadatas[cardID]
   }

   //Access to set metadata is controlled using an Admin reference as argument
   //
   pub fun setMetadata(adminRef: &MotoGPAdmin.Admin, cardID: UInt64, name:String, description:String, imageUrl:String, data:{String: String}) {
       pre {
           adminRef != nil: "adminRef is nil"
       }
       let metadata = Metadata(_cardID:cardID, _name:name, _description:description, _imageUrl:imageUrl, _data:data)
       self.metadatas[cardID] = metadata
   }

   //Remove metadata by cardID
   //
   pub fun removeMetadata(adminRef: &MotoGPAdmin.Admin, cardID: UInt64) {
       pre {
           adminRef != nil: "adminRef is nil"
       }
       self.metadatas.remove(key: cardID)
   }
   
   init(){
        self.metadatas = {}
   }
        
}
