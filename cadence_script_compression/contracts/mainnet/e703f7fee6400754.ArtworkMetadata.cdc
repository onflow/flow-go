// SPDX-License-Identifier: MIT

// This contracts contains Metadata structs for Artwork metadata

pub contract ArtworkMetadata {
	pub struct Creator {
		pub let name: String
		pub let bio: String
		// Creator Flow Address
		pub let address: Address
		// Link to Everbloom profile
		pub let externalLink: String?

		init(name: String, bio: String, address: Address, externalLink: String?) {
			self.name = name
			self.bio = bio
			self.address = address
			self.externalLink = externalLink
		}
	}

	pub struct Attribute {
		pub let traitType: String
		pub let value: String

		init(traitType: String, value: String){
			self.traitType = traitType
			self.value = value
		}
	}

	pub struct Content {
		pub let name: String
		pub let description: String
		// Link to the image
		pub let image: String
		pub let thumbnail: String
		pub let animation: String?
		// Link to Everbloom Post
		pub let externalLink: String?

		init (
			name: String,
			description: String,
			image: String,
			thumbnail: String,
			animation: String?,
			externalLink: String?
		) {
			self.name = name
			self.description = description
			self.image = image
			self.thumbnail = thumbnail
			self.animation = animation
			self.externalLink = externalLink
		}
	}
}
