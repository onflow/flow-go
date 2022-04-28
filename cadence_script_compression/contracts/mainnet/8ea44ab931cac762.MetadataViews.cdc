/**

This contract implements the metadata standard proposed
in FLIP-0636.

Ref: https://github.com/onflow/flow/blob/master/flips/20210916-nft-metadata.md

Structs and resources can implement one or more
metadata types, called views. Each view type represents
a different kind of metadata, such as a creator biography
or a JPEG image file.
*/

pub contract MetadataViews {

    // A Resolver provides access to a set of metadata views.
    //
    // A struct or resource (e.g. an NFT) can implement this interface
    // to provide access to the views that it supports.
    //
    pub resource interface Resolver {
        pub fun getViews(): [Type]
        pub fun resolveView(_ view: Type): AnyStruct?
    }

    // A ResolverCollection is a group of view resolvers index by ID.
    //
    pub resource interface ResolverCollection {
        pub fun borrowViewResolver(id: UInt64): &{Resolver}
        pub fun getIDs(): [UInt64]
    }

    // Display is a basic view that includes the name and description
    // of an object. Most objects should implement this view.
    //
    pub struct Display {
        pub let name: String
        pub let description: String

        init(
            name: String,
            description: String
        ) {
            self.name = name
            self.description = description
        }
    }

    // HTTPThumbnail returns a thumbnail image for an object.
    //
    pub struct HTTPThumbnail {
        pub let uri: String
        pub let mimetype: String

        init(
            uri: String,
            mimetype: String
        ) {
            self.uri = uri
            self.mimetype = mimetype
        }
    }

    // IPFSThumbnail returns a thumbnail image for an object
    // stored as an image file in IPFS.
    //
    // IPFS images are referenced by their content identifier (CID)
    // rather than a direct URI. A client application can use this CID
    // to find and load the image via an IPFS gateway.
    //
    pub struct IPFSThumbnail {
        pub let cid: String
        pub let mimetype: String

        init(
            cid: String,
            mimetype: String
        ) {
            self.cid = cid
            self.mimetype = mimetype
        }
    }
}