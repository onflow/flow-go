pub contract StarlyMetadataViews {

    pub struct Creator {
        pub let id: String
        pub let name: String
        pub let username: String
        pub let address: Address?
        pub let url: String

        init(
            id: String,
            name: String,
            username: String,
            address: Address?,
            url: String) {

            self.id = id
            self.name = name
            self.username = username
            self.address = address
            self.url = url
        }
    }

    pub struct Collection {
        pub let id: String
        pub let creator: Creator
        pub let title: String
        pub let description: String
        pub let priceCoefficient: UFix64
        pub let url: String

        init(
            id: String,
            creator: Creator,
            title: String,
            description: String,
            priceCoefficient: UFix64,
            url: String) {

            self.id = id
            self.creator = creator
            self.title = title
            self.description = description
            self.priceCoefficient = priceCoefficient
            self.url = url
        }
    }

    pub struct Card {
        pub let id: UInt32
        pub let title: String
        pub let description: String
        pub let editions: UInt32
        pub let rarity: String
        pub let mediaType: String
        pub let mediaSizes: [MediaSize]
        pub let url: String
        pub let previewUrl: String

        init(
            id: UInt32
            title: String
            description: String
            editions: UInt32
            rarity: String
            mediaType: String
            mediaSizes: [MediaSize]
            url: String,
            previewUrl: String) {

            self.id = id
            self.title = title
            self.description = description
            self.editions = editions
            self.rarity = rarity
            self.mediaType = mediaType
            self.mediaSizes = mediaSizes
            self.url = url
            self.previewUrl = previewUrl
        }
    }

    pub struct MediaSize {
        pub let width: UInt16
        pub let height: UInt16
        pub let url: String
        pub let screenshot: String?

        init(
            width: UInt16,
            height: UInt16,
            url: String,
            screenshot: String?) {

            self.width = width
            self.height = height
            self.url = url
            self.screenshot = screenshot
        }
    }

    pub struct CardEdition {
        pub let collection: Collection
        pub let card: Card
        pub let edition: UInt32
        pub let score: UInt32?
        pub let url: String
        pub let previewUrl: String

        init(
            collection: Collection,
            card: Card,
            edition: UInt32,
            score: UInt32?,
            url: String,
            previewUrl: String) {

            self.collection = collection
            self.card = card
            self.edition = edition
            self.score = score
            self.url = url
            self.previewUrl = previewUrl
        }
    }
}
