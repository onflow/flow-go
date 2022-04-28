
import FungibleToken from 0xf233dcee88fe0abe

pub contract NeoViews {

	pub struct StickerView {
		pub let id: UInt64
		pub let name: String
		pub let thumbnailHash: String
		pub let description: String
		pub let rarity:UInt64 
		pub let location:UInt64 
		pub let edition:UInt64 
		pub let maxEdition:UInt64
		pub let typeId:UInt64
		pub let setId:UInt64

		pub init(id: UInt64, name: String, description:String, thumbnailHash:String, rarity:UInt64, location:UInt64, edition:UInt64, maxEdition:UInt64, typeId:UInt64, setId:UInt64) {
			self.id=id
			self.name=name
			self.thumbnailHash=thumbnailHash
			self.description=description
			self.rarity=rarity
			self.location=location
			self.edition=edition
			self.maxEdition=maxEdition
			self.typeId=typeId
			self.setId=setId
		}
	}

	pub struct Royalties {
		pub let royalties: {String:Royalty}

		pub init(royalties: {String:Royalty}){
			self.royalties=royalties
		}
	}

	/// Since the royalty discussion has not been finalized yet we use a temporary royalty view here, we can later add an adapter to emit the propper one
	pub struct Royalty{
		pub let wallet:Capability<&{FungibleToken.Receiver}>
		pub let cut: UFix64

		init(wallet:Capability<&{FungibleToken.Receiver}>, cut: UFix64 ){
			self.wallet=wallet
			self.cut=cut
		}
	}

	pub struct ExternalDomainViewUrl {
  
		pub let url:String

		init(url: String) {
			self.url=url
		}
	}
}
