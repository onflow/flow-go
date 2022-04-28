/**

This contract implements the metadata standard proposed
in FLIP-0636.

Ref: https://github.com/onflow/flow/blob/master/flips/20210916-nft-metadata.md

Structs and resources can implement one or more
metadata types, called views. Each view type represents
a different kind of metadata, such as a creator biography
or a JPEG image file.
*/

import FungibleToken from 0xf233dcee88fe0abe

pub contract MetadataViews {

    /// A Resolver provides access to a set of metadata views.
    ///
    /// A struct or resource (e.g. an NFT) can implement this interface
    /// to provide access to the views that it supports.
    ///
    pub resource interface Resolver {
        pub fun getViews(): [Type]
        pub fun resolveView(_ view: Type): AnyStruct?
    }

    /// A ResolverCollection is a group of view resolvers index by ID.
    ///
    pub resource interface ResolverCollection {
        pub fun borrowViewResolver(id: UInt64): &{Resolver}
        pub fun getIDs(): [UInt64]
    }

    /// Display is a basic view that includes the name, description and
    /// thumbnail for an object. Most objects should implement this view.
    ///
    pub struct Display {

        /// The name of the object. 
        ///
        /// This field will be displayed in lists and therefore should
        /// be short an concise.
        ///
        pub let name: String

        /// A written description of the object. 
        ///
        /// This field will be displayed in a detailed view of the object,
        /// so can be more verbose (e.g. a paragraph instead of a single line).
        ///
        pub let description: String

        /// A small thumbnail representation of the object.
        ///
        /// This field should be a web-friendly file (i.e JPEG, PNG)
        /// that can be displayed in lists, link previews, etc.
        ///
        pub let thumbnail: AnyStruct{File}

        init(
            name: String,
            description: String,
            thumbnail: AnyStruct{File}
        ) {
            self.name = name
            self.description = description
            self.thumbnail = thumbnail
        }
    }

    /// File is a generic interface that represents a file stored on or off chain.
    ///
    /// Files can be used to references images, videos and other media.
    ///
    pub struct interface File {
        pub fun uri(): String
    }

    /// HTTPFile is a file that is accessible at an HTTP (or HTTPS) URL. 
    ///
    pub struct HTTPFile: File {
        pub let url: String

        init(url: String) {
            self.url = url
        }

        pub fun uri(): String {
            return self.url
        }
    }

    /// IPFSFile returns a thumbnail image for an object
    /// stored as an image file in IPFS.
    ///
    /// IPFS images are referenced by their content identifier (CID)
    /// rather than a direct URI. A client application can use this CID
    /// to find and load the image via an IPFS gateway.
    ///
    pub struct IPFSFile: File {

        /// CID is the content identifier for this IPFS file.
        ///
        /// Ref: https://docs.ipfs.io/concepts/content-addressing/
        ///
        pub let cid: String

        /// Path is an optional path to the file resource in an IPFS directory.
        ///
        /// This field is only needed if the file is inside a directory.
        ///
        /// Ref: https://docs.ipfs.io/concepts/file-systems/
        ///
        pub let path: String?

        init(cid: String, path: String?) {
            self.cid = cid
            self.path = path
        }

        /// This function returns the IPFS native URL for this file.
        ///
        /// Ref: https://docs.ipfs.io/how-to/address-ipfs-on-web/#native-urls
        ///
        pub fun uri(): String {
            if let path = self.path {
                return "ipfs://".concat(self.cid).concat("/").concat(path)
            }

            return "ipfs://".concat(self.cid)
        }
    }

    /*
    *  Royalty Views
    *  Defines the composable royalty standard that gives marketplaces a unified interface
    *  to support NFT royalties.
    *
    *  Marketplaces can query this `Royalties` struct from NFTs 
    *  and are expected to pay royalties based on these specifications.
    *
    */
    pub struct Royalties {

        /// Array that tracks the individual royalties
        access(self) let cutInfos: [Royalty]

        pub init(_ cutInfos: [Royalty]) {
            // Validate that sum of all cut multipliers should not be greater than 1.0
            var totalCut = 0.0
            for royalty in cutInfos {
                totalCut = totalCut + royalty.cut
            }
            assert(totalCut <= 1.0, message: "Sum of cutInfos multipliers should not be greater than 1.0")
            // Assign the cutInfos
            self.cutInfos = cutInfos
        }

        /// Return the cutInfos list
        pub fun getRoyalties(): [Royalty] {
            return self.cutInfos
        }
    }

    /// Struct to store details of a single royalty cut for a given NFT
    pub struct Royalty {

        /// Generic FungibleToken Receiver for the beneficiary of the royalty
        /// Can get the concrete type of the receiver with receiver.getType()
        /// Recommendation - Users should create a new link for a FlowToken receiver for this using `getRoyaltyReceiverPublicPath()`,
        /// and not use the default FlowToken receiver.
        /// This will allow users to update the capability in the future to use a more generic capability
        pub let receiver: Capability<&AnyResource{FungibleToken.Receiver}>

        /// Multiplier used to calculate the amount of sale value transferred to royalty receiver.
        /// Note - It should be between 0.0 and 1.0 
        /// Ex - If the sale value is x and multiplier is 0.56 then the royalty value would be 0.56 * x.
        ///
        /// Generally percentage get represented in terms of basis points
        /// in solidity based smart contracts while cadence offers `UFix64` that already supports
        /// the basis points use case because its operations
        /// are entirely deterministic integer operations and support up to 8 points of precision.
        pub let cut: UFix64

        /// Optional description: This can be the cause of paying the royalty,
        /// the relationship between the `wallet` and the NFT, or anything else that the owner might want to specify
        pub let description: String

        init(recepient: Capability<&AnyResource{FungibleToken.Receiver}>, cut: UFix64, description: String) {
            pre {
                cut >= 0.0 && cut <= 1.0 : "Cut value should be in valid range i.e [0,1]"
            }
            self.receiver = recepient
            self.cut = cut
            self.description = description
        }
    }

    /// Get the path that should be used for receiving royalties
    /// This is a path that will eventually be used for a generic switchboard receiver,
    /// hence the name but will only be used for royalties for now.
    pub fun getRoyaltyReceiverPublicPath(): PublicPath {
        return /public/GenericFTReceiver
    }

    // A view to represent Media, a file with an correspoiding mediaType.
    pub struct Media {
        pub let file: AnyStruct{File}

        // media-type comes on the form of type/subtype as described here https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types
        pub let mediaType: String

        init(file: AnyStruct{File}, mediaType: String) {
          self.file=file
          self.mediaType=mediaType
        }
    }

    // A license according to https://spdx.org/licenses/
    //
    // This view can be used if the content of an NFT is licensed. 
    pub struct License {
        pub let spdxIdentifier: String

        init(_ identifier: String) {
            self.spdxIdentifier = identifier
        }
    }

    // A view to expose a URL to this item on an external site.
    //
    // This can be used by applications like .find and Blocto to direct users to the original link for an NFT.
    pub struct ExternalURL {
        pub let url: String

        init(_ url: String) {
            self.url=url
        }
    }
}
