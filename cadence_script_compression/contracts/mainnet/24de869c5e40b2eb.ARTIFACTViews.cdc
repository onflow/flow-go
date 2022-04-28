import MetadataViews from 0x1d7e57aa55817448

pub contract ARTIFACTViews {

    // Display is a basic artifacts view that includes the name, description,
    // thumbnail for an object and metadata as flexible field. Most objects should implement this view.
    //
    pub struct ArtifactsDisplay {
        pub let name: String
        pub let description: String
        pub let thumbnail: AnyStruct{MetadataViews.File}
        pub let metadata: {String : String}

        init(
            name: String,
            description: String,
            thumbnail: AnyStruct{MetadataViews.File},
            metadata: {String : String}
        ) {
            self.name = name
            self.description = description
            self.thumbnail = thumbnail
            self.metadata = metadata
        }
    }
}